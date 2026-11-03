// Cliente del Election Service (agr-election-lo): lo que la saga y la
// composición de resultados necesitan de una elección.

import type { Config } from "../config.ts";
import { HttpError } from "../http.ts";
import { callUpstream, jsonOf, UpstreamError } from "./upstream.ts";

const SERVICE = "Election Service";

export interface Candidate {
  id: number;
  number: number;
  name: string;
}

export interface Election {
  id: number;
  name: string;
  isOpen: boolean;
  candidates: Candidate[];
}

function unavailable(): HttpError {
  return new HttpError(502, `El ${SERVICE} no responde`);
}

export class ElectionClient {
  private readonly cfg: Config;

  constructor(cfg: Config) {
    this.cfg = cfg;
  }

  async get(electionId: number): Promise<Election> {
    let res;
    try {
      res = await callUpstream({
        service: SERVICE,
        url: `${this.cfg.electionUrl}/elections/${electionId}`,
        timeoutMs: this.cfg.upstreamTimeoutMs,
      });
    } catch (err) {
      if (err instanceof UpstreamError) throw unavailable();
      throw err;
    }

    if (res.status === 404) {
      throw new HttpError(404, "La elección no existe");
    }
    if (res.status !== 200) {
      throw unavailable();
    }

    const data = jsonOf(res) as Record<string, unknown> | null;
    if (!data || !Array.isArray(data.candidates)) {
      throw unavailable();
    }
    const candidates: Candidate[] = (data.candidates as Record<string, unknown>[]).map((c, i) => ({
      id: Number(c.id),
      number: typeof c.number === "number" ? c.number : i + 1,
      name: String(c.name ?? ""),
    }));
    return {
      id: Number(data.id ?? electionId),
      name: String(data.name ?? ""),
      isOpen: data.is_open === true,
      candidates,
    };
  }
}
