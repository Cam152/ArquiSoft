// Llamadas que hace el navegador directo al gateway (NEXT_PUBLIC_GATEWAY_URL).
// Son solo dos, como pide la arquitectura: enviar el voto y abrir el SSE.
// La URL pública llega como prop desde el servidor, que la lee al ejecutar.

import { ApiError, messageFor, readDetail } from "./errors";

/** Tiempo máximo para la saga completa del voto (cuatro servicios y un reintento). */
const VOTE_TIMEOUT_MS = 30_000;

/** POST /votes con el JWT en Authorization. Resuelve si el gateway responde 201. */
export async function castVote(
  gatewayUrl: string,
  token: string,
  electionId: number,
  candidateId: number,
): Promise<void> {
  let response: Response;
  try {
    response = await fetch(`${gatewayUrl}/votes`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ election_id: electionId, candidate_id: candidateId }),
      signal: AbortSignal.timeout(VOTE_TIMEOUT_MS),
    });
  } catch {
    throw new ApiError(0, messageFor("vote", 0, null));
  }
  if (response.ok) return;

  const detail = await readDetail(response);
  throw new ApiError(response.status, messageFor("vote", response.status, detail), detail);
}

/** GET /results/{id}/stream: resultados en vivo (Server-Sent Events). */
export function resultsStreamUrl(gatewayUrl: string, electionId: number): string {
  return `${gatewayUrl}/results/${electionId}/stream`;
}
