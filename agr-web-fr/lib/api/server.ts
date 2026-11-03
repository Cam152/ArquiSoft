import "server-only";

// Cliente del gateway para el servidor de Next.js (SSR y Server Actions).
// Llama a agr-gateway-or por la red interna (GATEWAY_INTERNAL_URL); el navegador
// nunca ve estas peticiones. `server-only` hace fallar la compilación si un
// componente de cliente importa este archivo por error.

import { gatewayInternalUrl, gatewayTimeoutMs } from "@/lib/config";
import { ApiError, messageFor, readDetail, type Operation } from "./errors";
import type { Election, ElectionResults, LoginResponse } from "./types";

/** Log de cada llamada: método, ruta, código y duración. Nunca cuerpos ni tokens. */
function logCall(method: string, path: string, status: number | string, started: number) {
  console.log(JSON.stringify({ component: "agr-web-fr", to: "agr-gateway-or", method, path, status, ms: Date.now() - started }));
}

async function callGateway(operation: Operation, path: string, init: RequestInit = {}): Promise<Response> {
  const method = init.method ?? "GET";
  const started = Date.now();
  let response: Response;
  try {
    response = await fetch(`${gatewayInternalUrl()}${path}`, {
      ...init,
      cache: "no-store",
      signal: AbortSignal.timeout(gatewayTimeoutMs()),
    });
  } catch (error) {
    // Si el servidor de Next.js no llega al gateway, el problema no es la conexión
    // del votante: se informa como servicio no disponible (503) o lento (504).
    const timedOut = error instanceof DOMException && error.name === "TimeoutError";
    const status = timedOut ? 504 : 503;
    logCall(method, path, timedOut ? "timeout" : "unreachable", started);
    throw new ApiError(status, messageFor(operation, status, null));
  }
  logCall(method, path, response.status, started);

  if (!response.ok) {
    const detail = await readDetail(response);
    throw new ApiError(response.status, messageFor(operation, response.status, detail), detail);
  }
  return response;
}

// ---------------------------------------------------------------------------
// Elecciones (GraphQL a través del gateway)
// ---------------------------------------------------------------------------

const ELECTION_FIELDS = `
  id
  name
  description
  status
  startsAt
  endsAt
  isOpen
  candidateCount
  candidates { id number name description }
`;

const ELECTIONS_QUERY = `query Elections { elections { ${ELECTION_FIELDS} } }`;
const ELECTION_QUERY = `query Election($id: Int!) { election(id: $id) { ${ELECTION_FIELDS} } }`;

async function graphql<T>(operation: Operation, query: string, variables?: Record<string, unknown>): Promise<T> {
  const response = await callGateway(operation, "/graphql", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const body = (await response.json()) as { data?: T | null; errors?: { message: string }[] };
  if (body.errors?.length || !body.data) {
    // GraphQL responde 200 aunque la consulta falle; el error viene en el cuerpo.
    throw new ApiError(502, messageFor(operation, 502, null), body.errors?.[0]?.message ?? null);
  }
  return body.data;
}

/** Todas las elecciones con sus candidatos anidados, en una sola consulta. */
export async function getElections(): Promise<Election[]> {
  const data = await graphql<{ elections: Election[] }>("elections", ELECTIONS_QUERY);
  return data.elections;
}

/** Una elección con sus candidatos, o null si no existe. */
export async function getElection(id: number): Promise<Election | null> {
  const data = await graphql<{ election: Election | null }>("election", ELECTION_QUERY, { id });
  return data.election;
}

// ---------------------------------------------------------------------------
// Resultados (REST, composición del gateway)
// ---------------------------------------------------------------------------

/** GET /results/{id}: conteos con nombres, ceros, porcentajes y orden. */
export async function getResults(id: number): Promise<ElectionResults> {
  const response = await callGateway("results", `/results/${id}`, {
    headers: { Accept: "application/json" },
  });
  return (await response.json()) as ElectionResults;
}

// ---------------------------------------------------------------------------
// Autenticación (la usa la Server Action de /ingresar)
// ---------------------------------------------------------------------------

/** POST /auth/login. 200 con el JWT; 401 si el documento o la clave no coinciden. */
export async function login(document: string, password: string): Promise<LoginResponse> {
  const response = await callGateway("login", "/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ document, password }),
  });
  return (await response.json()) as LoginResponse;
}
