// Cliente del Auth Service (agr-auth-lo).
//
// Acepta las dos formas de respuesta que existen hoy: la del contrato del README
// ({access_token}, {voter_id}) y la que implementa agr-auth-lo
// ({token, voterId, ...}, {valid, voterId, ...}). Hacia afuera el gateway siempre
// responde con la forma del contrato.

import type { Config } from "../config.ts";
import { HttpError } from "../http.ts";
import { callUpstream, jsonOf, UpstreamError } from "./upstream.ts";

const SERVICE = "Auth Service";

export interface LoginResult {
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
}

function unavailable(): HttpError {
  return new HttpError(502, `El ${SERVICE} no responde`);
}

export class AuthClient {
  private readonly cfg: Config;

  constructor(cfg: Config) {
    this.cfg = cfg;
  }

  async login(document: string, password: string): Promise<LoginResult> {
    let res;
    try {
      res = await callUpstream({
        service: SERVICE,
        url: `${this.cfg.authUrl}/auth/login`,
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ document, password }),
        timeoutMs: this.cfg.upstreamTimeoutMs,
      });
    } catch (err) {
      if (err instanceof UpstreamError) throw unavailable();
      throw err;
    }

    if (res.status === 401 || res.status === 403) {
      throw new HttpError(401, "Documento o clave incorrectos");
    }
    if (res.status === 400 || res.status === 422) {
      throw new HttpError(422, "Faltan el documento o la clave");
    }
    if (res.status !== 200) {
      throw unavailable();
    }

    const data = jsonOf(res) as Record<string, unknown> | null;
    const token = data?.access_token ?? data?.token;
    if (typeof token !== "string" || token === "") {
      throw unavailable();
    }
    const expires = typeof data?.expires_in === "number" ? data.expires_in : 3600;
    return { access_token: token, token_type: "Bearer", expires_in: expires };
  }

  /** Devuelve el id del votante del token, o lanza 401 si el token no sirve. */
  async validate(authorization: string): Promise<number> {
    let res;
    try {
      res = await callUpstream({
        service: SERVICE,
        url: `${this.cfg.authUrl}/auth/validate`,
        headers: { Authorization: authorization },
        timeoutMs: this.cfg.upstreamTimeoutMs,
      });
    } catch (err) {
      if (err instanceof UpstreamError) throw unavailable();
      throw err;
    }

    if (res.status === 401 || res.status === 403) {
      throw new HttpError(401, "Token inválido o expirado");
    }
    if (res.status !== 200) {
      throw unavailable();
    }

    const data = jsonOf(res) as Record<string, unknown> | null;
    if (data?.valid === false) {
      throw new HttpError(401, "Token inválido o expirado");
    }
    const voterId = data?.voter_id ?? data?.voterId;
    if (typeof voterId !== "number" || !Number.isInteger(voterId) || voterId <= 0) {
      throw unavailable();
    }
    return voterId;
  }
}
