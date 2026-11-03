// Cliente del Results Service (agr-results-lo): conteos por candidato.

import type { Config } from "../config.ts";
import { HttpError } from "../http.ts";
import { callUpstream, jsonOf, UpstreamError } from "./upstream.ts";

const SERVICE = "Results Service";

function unavailable(): HttpError {
  return new HttpError(502, `El ${SERVICE} no responde`);
}

/** candidate_id -> votos */
export type Counts = Map<number, number>;

/**
 * Acepta "counts" como objeto {"<candidate_id>": votos} (contrato del README) o
 * como lista [{candidate_id, votes}].
 */
export function parseCounts(data: unknown): Counts {
  const counts: Counts = new Map();
  const raw = (data as { counts?: unknown } | null)?.counts;
  if (Array.isArray(raw)) {
    for (const item of raw as Record<string, unknown>[]) {
      const id = Number(item.candidate_id);
      const votes = Number(item.votes);
      if (Number.isInteger(id) && Number.isFinite(votes)) counts.set(id, votes);
    }
  } else if (raw && typeof raw === "object") {
    for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
      const id = Number(key);
      const votes = Number(value);
      if (Number.isInteger(id) && Number.isFinite(votes)) counts.set(id, votes);
    }
  }
  return counts;
}

export class ResultsClient {
  private readonly cfg: Config;

  constructor(cfg: Config) {
    this.cfg = cfg;
  }

  async counts(electionId: number): Promise<Counts> {
    let res;
    try {
      res = await callUpstream({
        service: SERVICE,
        url: `${this.cfg.resultsUrl}/results/${electionId}`,
        timeoutMs: this.cfg.upstreamTimeoutMs,
      });
    } catch (err) {
      if (err instanceof UpstreamError) throw unavailable();
      throw err;
    }
    // Una elección sin votos todavía puede no existir en Redis.
    if (res.status === 404) return new Map();
    if (res.status !== 200) throw unavailable();
    return parseCounts(jsonOf(res));
  }
}
