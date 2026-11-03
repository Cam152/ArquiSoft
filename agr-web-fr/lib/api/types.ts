// Tipos de los contratos del gateway (README principal, sección 5.1).
// GraphQL usa camelCase y REST usa snake_case; se conservan tal cual para no traducir.

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

/** POST /auth/login → 200 */
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

/** GET /results/{id}: composición del gateway (conteos + nombres de candidatos). */
export interface ElectionResults {
  election_id: number;
  election_name: string;
  total_votes: number;
  results: CandidateResult[];
}

/** Dato de cada evento SSE `counts` de GET /results/{id}/stream. No trae nombres. */
export interface CountsEvent {
  election_id: number;
  total_votes: number;
  counts: Record<string, number>;
}
