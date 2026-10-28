// Cliente HTTP genérico hacia los servicios internos, con tiempo máximo.
// Distingue los casos que la saga del voto trata distinto:
//   - el servicio respondió (con cualquier código)          -> UpstreamResponse
//   - no aceptó la conexión (caído, nombre no resuelve)     -> UpstreamError "refused"
//     La petición nunca llegó: es seguro asumir que no hizo nada.
//   - no respondió a tiempo                                 -> UpstreamError "timeout"
//   - la conexión se cortó a mitad de camino                -> UpstreamError "unreachable"
//     En estos dos últimos no se sabe si alcanzó a procesar la petición.

export interface UpstreamResponse {
  status: number;
  contentType: string;
  body: Buffer;
}

export type FailureKind = "refused" | "timeout" | "unreachable";

export class UpstreamError extends Error {
  readonly kind: FailureKind;
  readonly service: string;

  constructor(kind: FailureKind, service: string, cause?: unknown) {
    super(`${service}: ${kind}`, { cause });
    this.kind = kind;
    this.service = service;
  }
}

const REFUSED_CODES = new Set(["ECONNREFUSED", "ENOTFOUND", "EAI_AGAIN", "EHOSTUNREACH"]);

function errorCode(err: unknown): string | undefined {
  let current: unknown = err;
  for (let depth = 0; depth < 4 && current && typeof current === "object"; depth++) {
    const code = (current as { code?: unknown }).code;
    if (typeof code === "string") return code;
    current = (current as { cause?: unknown }).cause;
  }
  return undefined;
}

export interface UpstreamRequest {
  service: string;
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: Buffer | string;
  timeoutMs: number;
}

export async function callUpstream(req: UpstreamRequest): Promise<UpstreamResponse> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), req.timeoutMs);
  try {
    const res = await fetch(req.url, {
      method: req.method ?? "GET",
      headers: req.headers,
      body: req.body,
      signal: controller.signal,
    });
    const body = Buffer.from(await res.arrayBuffer());
    return {
      status: res.status,
      contentType: res.headers.get("content-type") ?? "application/json",
      body,
    };
  } catch (err) {
    if (controller.signal.aborted) {
      throw new UpstreamError("timeout", req.service, err);
    }
    const code = errorCode(err);
    if (code && REFUSED_CODES.has(code)) {
      throw new UpstreamError("refused", req.service, err);
    }
    throw new UpstreamError("unreachable", req.service, err);
  } finally {
    clearTimeout(timer);
  }
}

/** Interpreta el cuerpo de una respuesta como JSON; devuelve null si no lo es. */
export function jsonOf(res: UpstreamResponse): unknown {
  try {
    return JSON.parse(res.body.toString("utf8"));
  } catch {
    return null;
  }
}

/** Extrae el "detail" de un error de otro servicio, si viene. */
export function detailOf(res: UpstreamResponse): string | null {
  const data = jsonOf(res);
  if (data && typeof data === "object" && "detail" in data) {
    const detail = (data as { detail: unknown }).detail;
    if (typeof detail === "string") return detail;
  }
  return null;
}
