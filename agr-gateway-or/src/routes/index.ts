// Tabla de rutas públicas del gateway (README, sección 5.1).

import type { IncomingMessage, ServerResponse } from "node:http";
import type { Config } from "../config.ts";
import { AuthClient } from "../clients/auth.ts";
import { ElectionClient } from "../clients/election.ts";
import { ResultsClient } from "../clients/results.ts";
import { VoteClient } from "../clients/vote.ts";
import { VoterClient } from "../clients/voter.ts";
import { composeResults } from "../compose/results.ts";
import { HttpError, parseJson, readBody, sendJson } from "../http.ts";
import { requireAdmin } from "../middleware/admin.ts";
import { castVote } from "../saga/vote.ts";
import { proxyResultsStream } from "../sse/proxy.ts";
import { forward } from "./forward.ts";

export interface RouteContext {
  req: IncomingMessage;
  res: ServerResponse;
  params: string[];
  query: string;
}

interface Route {
  method: string;
  pattern: RegExp;
  handler: (ctx: RouteContext) => Promise<void>;
}

function toId(raw: string): number {
  const id = Number(raw);
  if (!Number.isInteger(id) || id <= 0) throw new HttpError(404, "Recurso no encontrado");
  return id;
}

export function buildRoutes(cfg: Config, log: (message: string) => void): Route[] {
  const auth = new AuthClient(cfg);
  const elections = new ElectionClient(cfg);
  const voters = new VoterClient(cfg);
  const votes = new VoteClient(cfg);
  const results = new ResultsClient(cfg);
  const timeoutMs = cfg.upstreamTimeoutMs;

  const toElections = async (ctx: RouteContext, path: string, admin: boolean): Promise<void> => {
    const headers: Record<string, string> = {};
    const contentType = ctx.req.headers["content-type"];
    if (contentType) headers["Content-Type"] = contentType;
    const accept = ctx.req.headers.accept;
    if (accept) headers.Accept = accept;
    if (admin) headers["X-API-Key"] = cfg.adminApiKey;
    const body = ctx.req.method === "GET" ? undefined : await readBody(ctx.req, cfg.maxBodyBytes);
    await forward(ctx.res, {
      service: "Election Service",
      url: `${cfg.electionUrl}${path}${ctx.query}`,
      method: ctx.req.method ?? "GET",
      headers,
      body,
      timeoutMs,
    });
  };

  return [
    {
      method: "GET",
      pattern: /^\/health$/,
      handler: async ({ res }) => sendJson(res, 200, { status: "ok" }),
    },

    // ---- Autenticación ---------------------------------------------------
    {
      method: "POST",
      pattern: /^\/auth\/login$/,
      handler: async ({ req, res }) => {
        const data = parseJson(await readBody(req, cfg.maxBodyBytes)) as Record<string, unknown> | null;
        const document = typeof data?.document === "string" ? data.document.trim() : "";
        const password = typeof data?.password === "string" ? data.password : "";
        if (!document || !password) throw new HttpError(422, "Faltan el documento o la clave");
        sendJson(res, 200, await auth.login(document, password));
      },
    },

    // ---- Elecciones (lectura pública) -------------------------------------
    {
      method: "POST",
      pattern: /^\/graphql$/,
      handler: (ctx) => toElections(ctx, "/graphql", false),
    },
    {
      method: "GET",
      pattern: /^\/graphql$/,
      handler: (ctx) => toElections(ctx, "/graphql", false),
    },
    {
      method: "GET",
      pattern: /^\/elections$/,
      handler: (ctx) => toElections(ctx, "/elections", false),
    },
    {
      method: "GET",
      pattern: /^\/elections\/(\d+)$/,
      handler: (ctx) => toElections(ctx, `/elections/${toId(ctx.params[0])}`, false),
    },

    // ---- Voto (saga) ------------------------------------------------------
    {
      method: "POST",
      pattern: /^\/votes$/,
      handler: async ({ req, res }) => {
        const body = await readBody(req, cfg.maxBodyBytes);
        await castVote({ auth, elections, voters, votes, log }, req.headers.authorization, body);
        sendJson(res, 201, { status: "recorded" });
      },
    },

    // ---- Resultados -------------------------------------------------------
    {
      method: "GET",
      pattern: /^\/results\/(\d+)$/,
      handler: async ({ res, params }) => {
        sendJson(res, 200, await composeResults(elections, results, toId(params[0])));
      },
    },
    {
      method: "GET",
      pattern: /^\/results\/(\d+)\/stream$/,
      handler: ({ req, res, params }) => proxyResultsStream(cfg, toId(params[0]), req, res),
    },

    // ---- Administración (X-API-Key) ----------------------------------------
    {
      method: "POST",
      pattern: /^\/admin\/elections$/,
      handler: async (ctx) => {
        requireAdmin(ctx.req, cfg.adminApiKey);
        await toElections(ctx, "/elections", true);
      },
    },
    {
      method: "PATCH",
      pattern: /^\/admin\/elections\/(\d+)$/,
      handler: async (ctx) => {
        requireAdmin(ctx.req, cfg.adminApiKey);
        await toElections(ctx, `/elections/${toId(ctx.params[0])}`, true);
      },
    },
    {
      method: "DELETE",
      pattern: /^\/admin\/elections\/(\d+)$/,
      handler: async (ctx) => {
        requireAdmin(ctx.req, cfg.adminApiKey);
        await toElections(ctx, `/elections/${toId(ctx.params[0])}`, true);
      },
    },
    {
      method: "POST",
      pattern: /^\/admin\/elections\/(\d+)\/candidates$/,
      handler: async (ctx) => {
        requireAdmin(ctx.req, cfg.adminApiKey);
        await toElections(ctx, `/elections/${toId(ctx.params[0])}/candidates`, true);
      },
    },
    {
      method: "DELETE",
      pattern: /^\/admin\/elections\/(\d+)\/candidates\/(\d+)$/,
      handler: async (ctx) => {
        requireAdmin(ctx.req, cfg.adminApiKey);
        const path = `/elections/${toId(ctx.params[0])}/candidates/${toId(ctx.params[1])}`;
        await toElections(ctx, path, true);
      },
    },
    {
      method: "POST",
      pattern: /^\/admin\/elections\/(\d+)\/(activate|close)$/,
      handler: async (ctx) => {
        requireAdmin(ctx.req, cfg.adminApiKey);
        await toElections(ctx, `/elections/${toId(ctx.params[0])}/${ctx.params[1]}`, true);
      },
    },
    {
      method: "GET",
      pattern: /^\/admin\/audit$/,
      handler: async ({ req, res, query }) => {
        requireAdmin(req, cfg.adminApiKey);
        await forward(res, {
          service: "Audit Service",
          url: `${cfg.auditUrl}/audit/events${query}`,
          method: "GET",
          timeoutMs,
        });
      },
    },
  ];
}

/** Busca la ruta. Devuelve null si no existe; lanza 405 si existe con otro método. */
export function matchRoute(
  routes: Route[],
  method: string,
  path: string,
): { route: Route; params: string[] } | null {
  let pathExists = false;
  for (const route of routes) {
    const match = route.pattern.exec(path);
    if (!match) continue;
    pathExists = true;
    if (route.method === method) return { route, params: match.slice(1) };
  }
  if (pathExists) throw new HttpError(405, "Método no permitido");
  return null;
}
