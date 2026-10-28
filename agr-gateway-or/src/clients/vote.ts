// Cliente del Vote Service (agr-vote-lo). Envía el voto SIN ningún dato del
// votante: solo elección, candidato y su nombre, más la llave de idempotencia.

import type { Config } from "../config.ts";
import { callUpstream } from "./upstream.ts";
import type { UpstreamResponse } from "./upstream.ts";

export interface VoteToStore {
  electionId: number;
  candidateId: number;
  candidateName: string;
}

export class VoteClient {
  private readonly cfg: Config;

  constructor(cfg: Config) {
    this.cfg = cfg;
  }

  /**
   * Devuelve la respuesta del servicio, sea cual sea su código. Lanza
   * UpstreamError si no hubo respuesta: la saga decide qué hacer en cada caso.
   */
  store(vote: VoteToStore, idempotencyKey: string): Promise<UpstreamResponse> {
    return callUpstream({
      service: "Vote Service",
      url: `${this.cfg.voteUrl}/votes`,
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Service-Key": this.cfg.serviceApiKey,
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify({
        election_id: vote.electionId,
        candidate_id: vote.candidateId,
        candidate_name: vote.candidateName,
      }),
      timeoutMs: this.cfg.upstreamTimeoutMs,
    });
  }
}
