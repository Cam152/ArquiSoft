// Saga del voto (README, sección 6.1). El gateway coordina a cuatro servicios
// que no se conocen entre sí:
//
//   1. agr-auth-lo      valida el JWT y entrega el voter_id
//   2. agr-election-lo  la elección existe, está abierta y el candidato le pertenece
//   3. agr-voter-lo     marca la participación (409 si ya votó en esa elección)
//   4. agr-vote-lo      guarda el voto, SIN datos del votante
//
// Si el paso 4 falla de forma explícita, se compensa desmarcando (paso 3).
// Si el paso 4 no responde, se reintenta una vez con la misma Idempotency-Key;
// si sigue sin respuesta y no se sabe si el voto se guardó, NO se compensa,
// porque desmarcar permitiría votar dos veces.
//
// Anonimato: este módulo nunca registra en logs el cuerpo de la petición, el
// token ni el candidato.

import { randomUUID } from "node:crypto";
import type { AuthClient } from "../clients/auth.ts";
import type { ElectionClient } from "../clients/election.ts";
import type { VoterClient } from "../clients/voter.ts";
import type { VoteClient, VoteToStore } from "../clients/vote.ts";
import { UpstreamError } from "../clients/upstream.ts";
import type { UpstreamResponse } from "../clients/upstream.ts";
import { HttpError, isPositiveInt, parseJson } from "../http.ts";

export interface SagaDeps {
  auth: AuthClient;
  elections: ElectionClient;
  voters: VoterClient;
  votes: VoteClient;
  log: (message: string) => void;
}

interface VoteRequest {
  electionId: number;
  candidateId: number;
}

function parseVoteRequest(raw: Buffer): VoteRequest {
  const data = parseJson(raw) as Record<string, unknown> | null;
  if (!data || typeof data !== "object") {
    throw new HttpError(422, "El cuerpo debe ser un objeto JSON");
  }
  if (!isPositiveInt(data.election_id) || !isPositiveInt(data.candidate_id)) {
    throw new HttpError(422, "election_id y candidate_id deben ser enteros positivos");
  }
  return { electionId: data.election_id, candidateId: data.candidate_id };
}

type StoreAttempt = { ok: true } | { ok: false; response: UpstreamResponse } | { ok: false; failure: UpstreamError };

async function attemptStore(votes: VoteClient, vote: VoteToStore, key: string): Promise<StoreAttempt> {
  try {
    const response = await votes.store(vote, key);
    if (response.status === 200 || response.status === 201) return { ok: true };
    return { ok: false, response };
  } catch (err) {
    if (err instanceof UpstreamError) return { ok: false, failure: err };
    throw err;
  }
}

export async function castVote(deps: SagaDeps, authorization: string | undefined, rawBody: Buffer): Promise<void> {
  if (!authorization || !/^Bearer\s+\S+/i.test(authorization)) {
    throw new HttpError(401, "Falta el token de sesión");
  }
  const request = parseVoteRequest(rawBody);

  // 1. Identidad
  const voterId = await deps.auth.validate(authorization);

  // 2. Elección y candidato (antes de marcar, para no gastar el voto)
  const election = await deps.elections.get(request.electionId);
  if (!election.isOpen) {
    throw new HttpError(409, "La elección no está abierta");
  }
  const candidate = election.candidates.find((c) => c.id === request.candidateId);
  if (!candidate) {
    throw new HttpError(422, "El candidato no pertenece a esta elección");
  }

  // 3. Participación (atómica en agr-voter-lo)
  await deps.voters.markVoted(voterId, request.electionId);

  // 4. Guardar el voto, sin datos del votante
  const vote: VoteToStore = {
    electionId: request.electionId,
    candidateId: candidate.id,
    candidateName: candidate.name,
  };
  const key = randomUUID();

  let attempt = await attemptStore(deps.votes, vote, key);
  if (!attempt.ok && "failure" in attempt) {
    // Sin respuesta: un reintento con la misma llave no puede duplicar el voto.
    attempt = await attemptStore(deps.votes, vote, key);
  }
  if (attempt.ok) return;

  if ("response" in attempt) {
    // El servicio respondió que no guardó el voto: compensar.
    await compensate(deps, voterId, request.electionId);
    throw new HttpError(500, "No se pudo guardar tu voto y no quedó registrado. Puedes intentarlo de nuevo.");
  }

  if (attempt.failure.kind === "refused") {
    // La conexión nunca se abrió: el voto seguro no se guardó, así que compensar es seguro.
    await compensate(deps, voterId, request.electionId);
    throw new HttpError(502, "El Vote Service no responde. Tu voto no quedó registrado; intenta de nuevo en unos minutos.");
  }

  // Timeout o conexión cortada: el voto pudo haberse guardado. No se compensa.
  deps.log("saga: el Vote Service no respondió tras el reintento; no se compensa");
  throw new HttpError(502, "El sistema de votación no pudo confirmar tu voto en este momento.");
}

async function compensate(deps: SagaDeps, voterId: number, electionId: number): Promise<void> {
  const ok = await deps.voters.unmarkVoted(voterId, electionId);
  if (!ok) {
    deps.log("saga: falló la compensación (unmark-voted)");
  }
}
