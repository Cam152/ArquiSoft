// Combina los conteos con la lista de candidatos. El SSE solo trae
// {candidate_id: votos}; los nombres y el número de tarjetón llegan con la página.
// Calcula igual que la composición del gateway (compose/results.ts): total = suma
// de los candidatos conocidos, porcentaje con dos decimales y orden por votos y
// luego por número de tarjetón.

import type { CandidateResult } from "./api/types";

export interface CandidateRef {
  id: number;
  number: number;
  name: string;
}

export interface TallyRow extends CandidateResult {
  number: number;
}

export interface Tally {
  total: number;
  rows: TallyRow[];
}

/** candidate_id → votos */
export type Votes = Record<number, number>;

export function tally(candidates: readonly CandidateRef[], votes: Votes): Tally {
  const rows: TallyRow[] = candidates.map((c) => ({
    candidate_id: c.id,
    candidate_name: c.name,
    number: c.number,
    votes: votes[c.id] ?? 0,
    percentage: 0,
  }));
  const total = rows.reduce((sum, row) => sum + row.votes, 0);
  for (const row of rows) {
    row.percentage = total > 0 ? Math.round((row.votes * 10000) / total) / 100 : 0;
  }
  rows.sort((a, b) => b.votes - a.votes || a.number - b.number);
  return { total, rows };
}

/** Votos iniciales a partir de la respuesta de GET /results/{id}. */
export function votesFromResults(results: readonly CandidateResult[]): Votes {
  const votes: Votes = {};
  for (const r of results) votes[r.candidate_id] = r.votes;
  return votes;
}

/**
 * Interpreta el `data` de un evento SSE `counts`. Devuelve null si no es JSON
 * válido o si es de otra elección, para ignorarlo sin romper la página.
 */
export function parseCountsEvent(raw: string, electionId: number): Votes | null {
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!data || typeof data !== "object") return null;
  const { election_id, counts } = data as { election_id?: unknown; counts?: unknown };
  if (election_id !== electionId || !counts || typeof counts !== "object") return null;

  const votes: Votes = {};
  for (const [key, value] of Object.entries(counts as Record<string, unknown>)) {
    const id = Number(key);
    if (Number.isInteger(id) && id > 0 && typeof value === "number" && Number.isFinite(value) && value >= 0) {
      votes[id] = value;
    }
  }
  return votes;
}
