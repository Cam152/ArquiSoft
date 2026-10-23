// Clientes HTTP de los tres servicios (contratos de la sección 6 del README).
// - Election Service: GraphQL, lectura de elecciones y candidatos.
// - Voter Service:    REST, inicio de sesión.
// - Vote Service:     REST, emitir el voto y consultar resultados.
// Las URLs se fijan al compilar (variables VITE_*) y son las que ve el navegador,
// por eso apuntan a localhost y no al nombre del contenedor.

const ELECTION_API_URL: string =
  import.meta.env.VITE_ELECTION_API_URL ?? "http://localhost:8000";
const VOTER_API_URL: string = import.meta.env.VITE_VOTER_API_URL ?? "http://localhost:8001";
const VOTE_API_URL: string = import.meta.env.VITE_VOTE_API_URL ?? "http://localhost:8002";

// ---------------------------------------------------------------------------
// Tipos
// ---------------------------------------------------------------------------

export type ElectionStatus = "draft" | "active" | "closed";

export interface Candidate {
  id: number;
  number: number;
  name: string;
  description: string | null;
}

export interface Election {
  id: number;
  name: string;
  description: string | null;
  status: ElectionStatus;
  startsAt: string | null;
  endsAt: string | null;
  isOpen: boolean;
  candidateCount: number;
  candidates: Candidate[];
}

// Los contratos REST usan snake_case; se conserva tal cual para no tener que traducir.
export interface LoginResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
}

export interface CandidateResult {
  candidate_id: number;
  candidate_name: string;
  votes: number;
  percentage: number;
}

export interface ElectionResults {
  election_id: number;
  total_votes: number;
  results: CandidateResult[];
}

// ---------------------------------------------------------------------------
// Errores
// ---------------------------------------------------------------------------

/** Error de cualquier llamada a los servicios. `status` es 0 si no hubo respuesta (red caída o CORS). */
export class ApiError extends Error {
  readonly status: number;
  readonly detail: string | null;

  constructor(status: number, message: string, detail: string | null = null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.detail = detail;
  }
}

export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  if (error instanceof Error) return new ApiError(0, error.message);
  return new ApiError(0, "Ocurrió un error inesperado.");
}

/** Mensaje fijo, o función que recibe el `detail` del servidor para decidir el mensaje. */
type ErrorMessage = string | ((detail: string | null) => string);
type ErrorMessages = Partial<Record<number, ErrorMessage>>;

const NETWORK_ERROR =
  "No se pudo conectar con el servidor. Revisa que los servicios estén en ejecución.";

const DEFAULT_MESSAGES: ErrorMessages = {
  502: "Un servicio del sistema no responde. Intenta de nuevo en unos minutos.",
  503: "El servicio no está disponible en este momento. Intenta de nuevo en unos minutos.",
};

/** Lee `{"detail": "..."}`, la forma de error común a todos los servicios. */
async function readDetail(response: Response): Promise<string | null> {
  try {
    const body: unknown = await response.json();
    if (body && typeof body === "object" && "detail" in body) {
      const { detail } = body as { detail: unknown };
      if (typeof detail === "string") return detail;
    }
  } catch {
    // Cuerpo vacío o que no es JSON: se usa el mensaje por código.
  }
  return null;
}

async function request<T>(url: string, init: RequestInit, messages: ErrorMessages = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new ApiError(0, NETWORK_ERROR);
  }

  if (response.ok) {
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  }

  const detail = await readDetail(response);
  const custom = messages[response.status] ?? DEFAULT_MESSAGES[response.status];
  const message =
    typeof custom === "function"
      ? custom(detail)
      : custom ?? detail ?? `Error inesperado del servidor (HTTP ${response.status}).`;
  throw new ApiError(response.status, message, detail);
}

// ---------------------------------------------------------------------------
// Election Service (GraphQL)
// ---------------------------------------------------------------------------

const ELECTIONS_QUERY = `
  query Elections {
    elections {
      id
      name
      description
      status
      startsAt
      endsAt
      isOpen
      candidateCount
      candidates { id number name description }
    }
  }
`;

export async function fetchElections(): Promise<Election[]> {
  let response: Response;
  try {
    response = await fetch(`${ELECTION_API_URL}/graphql`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query: ELECTIONS_QUERY }),
    });
  } catch {
    throw new ApiError(0, NETWORK_ERROR);
  }
  if (!response.ok) {
    throw new ApiError(
      response.status,
      `El Election Service respondió con HTTP ${response.status}`,
    );
  }
  const body = await response.json();
  if (body.errors?.length) {
    throw new ApiError(response.status, body.errors[0].message);
  }
  return body.data.elections as Election[];
}

/** Una elección con sus candidatos. Reutiliza la consulta de la lista para no depender de otro campo del esquema. */
export async function fetchElection(electionId: number): Promise<Election> {
  const elections = await fetchElections();
  const election = elections.find((e) => e.id === electionId);
  if (!election) throw new ApiError(404, "La elección no existe.");
  return election;
}

// ---------------------------------------------------------------------------
// Voter Service (REST, puerto 8001)
// ---------------------------------------------------------------------------

/** POST /auth/login. 200 con el JWT, 401 si el documento o la clave no coinciden. */
export function login(document: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>(
    `${VOTER_API_URL}/auth/login`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ document, password }),
    },
    {
      401: "Documento o clave incorrectos.",
      422: "Revisa el documento y la clave.",
    },
  );
}

// ---------------------------------------------------------------------------
// Vote Service (REST, puerto 8002)
// ---------------------------------------------------------------------------

/**
 * POST /votes con el JWT en Authorization. Responde 201 si el voto quedó registrado.
 * El 409 tiene dos causas posibles (ya votó, o la elección no está abierta); se distinguen
 * por el `detail`, que el Vote Service reenvía desde el Voter Service ("El votante ya votó").
 */
export async function castVote(
  token: string,
  electionId: number,
  candidateId: number,
): Promise<void> {
  await request<unknown>(
    `${VOTE_API_URL}/votes`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ election_id: electionId, candidate_id: candidateId }),
    },
    {
      401: "Tu sesión expiró, vuelve a ingresar.",
      404: "La elección no existe.",
      409: (detail) =>
        detail && /ya vot/i.test(detail)
          ? "Ya votaste. Cada votante puede votar una sola vez."
          : "La elección no está abierta.",
      422: "El candidato no pertenece a esta elección. Recarga la página e inténtalo de nuevo.",
      500: "No se pudo guardar tu voto y no quedó registrado. Puedes intentarlo de nuevo.",
      502: "El sistema de votación no puede validar tu voto en este momento. Intenta de nuevo en unos minutos.",
    },
  );
}

/** GET /results/{election_id}, público. Viene ordenado de más a menos votos. */
export function fetchResults(electionId: number): Promise<ElectionResults> {
  return request<ElectionResults>(
    `${VOTE_API_URL}/results/${electionId}`,
    { method: "GET" },
    { 404: "La elección no existe." },
  );
}
