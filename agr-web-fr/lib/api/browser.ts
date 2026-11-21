// Llamadas que hace el navegador al gateway a través de agr-proxy-rp
// (NEXT_PUBLIC_GATEWAY_URL, por defecto "/api": mismo origen, sin CORS).
// Son tres: iniciar sesión, enviar el voto y abrir el SSE de resultados.
// La URL pública llega como prop desde el servidor, que la lee al ejecutar.

import { ApiError, messageFor, readDetail } from "./errors";
import type { LoginResponse } from "./types";

/** Tiempo máximo para el login (gateway y Auth Service). */
const LOGIN_TIMEOUT_MS = 15_000;

/**
 * POST /auth/login desde el navegador. Pasa por /api/auth/login en el proxy, que
 * aplica el límite de intentos por IP (S4, 429) antes de llegar al gateway.
 */
export async function login(gatewayUrl: string, document: string, password: string): Promise<LoginResponse> {
  let response: Response;
  try {
    response = await fetch(`${gatewayUrl}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ document, password }),
      // El JWT no viaja en cookies: no hace falta enviar credenciales.
      credentials: "omit",
      signal: AbortSignal.timeout(LOGIN_TIMEOUT_MS),
    });
  } catch {
    throw new ApiError(0, messageFor("login", 0, null));
  }

  if (response.ok) {
    try {
      const body = (await response.json()) as Partial<LoginResponse>;
      if (typeof body.access_token === "string" && typeof body.expires_in === "number") {
        return body as LoginResponse;
      }
    } catch {
      // Respuesta que no es JSON: se trata abajo como error inesperado.
    }
    throw new ApiError(502, messageFor("login", 502, null));
  }

  const detail = await readDetail(response);
  throw new ApiError(response.status, messageFor("login", response.status, detail), detail);
}

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
