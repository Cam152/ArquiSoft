// Cliente del Voter Service (agr-voter-lo): marca y desmarca la participación
// de un votante en una elección. Ambas llamadas llevan X-Service-Key.

import type { Config } from "../config.ts";
import { HttpError } from "../http.ts";
import { callUpstream, UpstreamError } from "./upstream.ts";

const SERVICE = "Voter Service";

function unavailable(): HttpError {
  return new HttpError(502, `El ${SERVICE} no responde`);
}

export class VoterClient {
  private readonly cfg: Config;

  constructor(cfg: Config) {
    this.cfg = cfg;
  }

  private call(path: string, voterId: number, electionId: number) {
    return callUpstream({
      service: SERVICE,
      url: `${this.cfg.voterUrl}${path}`,
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Service-Key": this.cfg.serviceApiKey,
      },
      body: JSON.stringify({ voter_id: voterId, election_id: electionId }),
      timeoutMs: this.cfg.upstreamTimeoutMs,
    });
  }

  /** Marca la participación. Lanza 409 si ya votó en esa elección. */
  async markVoted(voterId: number, electionId: number): Promise<void> {
    let res;
    try {
      res = await this.call("/voters/mark-voted", voterId, electionId);
    } catch (err) {
      if (err instanceof UpstreamError) throw unavailable();
      throw err;
    }
    if (res.status === 409) {
      throw new HttpError(409, "Ya votaste en esta elección");
    }
    if (res.status !== 200 && res.status !== 201) {
      throw unavailable();
    }
  }

  /** Compensación de la saga. Devuelve false si no se pudo desmarcar. */
  async unmarkVoted(voterId: number, electionId: number): Promise<boolean> {
    try {
      const res = await this.call("/voters/unmark-voted", voterId, electionId);
      return res.status === 200 || res.status === 204;
    } catch {
      return false;
    }
  }
}
