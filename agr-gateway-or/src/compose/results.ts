// Composición de resultados (README, sección 5.1). agr-results-lo solo conoce
// conteos por candidato; agr-election-lo conoce los nombres y los candidatos sin
// votos. El gateway une ambos, porque los servicios de lógica no se llaman entre sí.

import type { ElectionClient, Election } from "../clients/election.ts";
import type { Counts, ResultsClient } from "../clients/results.ts";

export interface CandidateResult {
  candidate_id: number;
  candidate_name: string;
  votes: number;
  percentage: number;
}

export interface ElectionResults {
  election_id: number;
  election_name: string;
  total_votes: number;
  results: CandidateResult[];
}

export function buildResults(election: Election, counts: Counts): ElectionResults {
  const rows = election.candidates.map((c) => ({
    candidate_id: c.id,
    candidate_name: c.name,
    number: c.number,
    votes: counts.get(c.id) ?? 0,
  }));
  const total = rows.reduce((sum, r) => sum + r.votes, 0);
  rows.sort((a, b) => b.votes - a.votes || a.number - b.number);
  return {
    election_id: election.id,
    election_name: election.name,
    total_votes: total,
    results: rows.map((r) => ({
      candidate_id: r.candidate_id,
      candidate_name: r.candidate_name,
      votes: r.votes,
      percentage: total > 0 ? Math.round((r.votes * 10000) / total) / 100 : 0,
    })),
  };
}

export async function composeResults(
  elections: ElectionClient,
  results: ResultsClient,
  electionId: number,
): Promise<ElectionResults> {
  // En paralelo. Si la elección no existe, ese error manda sobre el de resultados.
  const [election, counts] = await Promise.allSettled([elections.get(electionId), results.counts(electionId)]);
  if (election.status === "rejected") throw election.reason;
  if (counts.status === "rejected") throw counts.reason;
  return buildResults(election.value, counts.value);
}
