// Pruebas del gateway contra servicios simulados. No necesitan Docker:
//   node --test test/

import assert from "node:assert/strict";
import { createServer } from "node:http";
import type { IncomingMessage, Server, ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import { after, before, beforeEach, describe, test } from "node:test";
import { createGateway } from "../src/app.ts";
import type { Config } from "../src/config.ts";

// ---------------------------------------------------------------------------
// Servicios simulados
// ---------------------------------------------------------------------------

interface Recorded {
  method: string;
  path: string;
  headers: IncomingMessage["headers"];
  body: string;
}

type Handler = (req: Recorded, res: ServerResponse) => void | Promise<void>;

class FakeService {
  server: Server;
  calls: Recorded[] = [];
  handler: Handler = (_req, res) => json(res, 404, { detail: "no configurado" });

  constructor() {
    this.server = createServer(async (req, res) => {
      const chunks: Buffer[] = [];
      for await (const c of req) chunks.push(c as Buffer);
      const rec: Recorded = {
        method: req.method ?? "",
        path: req.url ?? "",
        headers: req.headers,
        body: Buffer.concat(chunks).toString("utf8"),
      };
      this.calls.push(rec);
      await this.handler(rec, res);
    });
  }

  async start(): Promise<string> {
    await new Promise<void>((r) => this.server.listen(0, "127.0.0.1", r));
    return `http://127.0.0.1:${(this.server.address() as AddressInfo).port}`;
  }

  stop(): Promise<void> {
    this.server.closeAllConnections();
    return new Promise((r) => this.server.close(() => r()));
  }

  paths(): string[] {
    return this.calls.map((c) => `${c.method} ${c.path}`);
  }
}

function json(res: ServerResponse, status: number, body: unknown): void {
  res.statusCode = status;
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(body));
}

const ELECTION_1 = {
  id: 1,
  name: "Elección de representante estudiantil 2026",
  status: "active",
  is_open: true,
  candidates: [
    { id: 1, election_id: 1, number: 1, name: "Ana Torres" },
    { id: 2, election_id: 1, number: 2, name: "Luis Herrera" },
    { id: 3, election_id: 1, number: 3, name: "Camila Duarte" },
  ],
};

const auth = new FakeService();
const election = new FakeService();
const voter = new FakeService();
const vote = new FakeService();
const results = new FakeService();
const audit = new FakeService();
let gateway: Server;
let base = "";
const logs: string[] = [];

function defaultHandlers(): void {
  // agr-auth-lo tal como lo implementó Esteban: {token,...} y {valid, voterId,...}
  auth.handler = (req, res) => {
    if (req.path === "/auth/login") {
      const { document, password } = JSON.parse(req.body);
      if (document === "1001" && password === "123456") {
        return json(res, 200, { token: "jwt-ok", voterId: 7, name: "Ana", document: "1001" });
      }
      return json(res, 500, { error: "Internal Server Error" });
    }
    if (req.path === "/auth/validate") {
      if (req.headers.authorization === "Bearer jwt-ok") {
        return json(res, 200, { valid: true, voterId: 7, document: "1001", error: null });
      }
      return json(res, 401, { valid: false, voterId: null, error: "Token expirado" });
    }
    json(res, 404, {});
  };
  election.handler = (req, res) => {
    if (req.method === "GET" && req.path === "/elections/1") return json(res, 200, ELECTION_1);
    if (req.method === "GET" && req.path === "/elections/3") return json(res, 200, { ...ELECTION_1, id: 3, is_open: false });
    if (req.method === "GET" && /^\/elections(\?|$)/.test(req.path)) return json(res, 200, [ELECTION_1]);
    if (req.path === "/graphql") return json(res, 200, { data: { elections: [{ id: 1 }] } });
    if (req.method === "POST" && req.path === "/elections") return json(res, 201, { id: 4 });
    json(res, 404, { detail: "La elección no existe" });
  };
  voter.handler = (req, res) => json(res, 200, { status: req.path.endsWith("unmark-voted") ? "unmarked" : "marked" });
  vote.handler = (_req, res) => json(res, 201, { status: "recorded" });
  results.handler = (req, res) => {
    if (req.path === "/results/1") return json(res, 200, { election_id: 1, total_votes: 10, counts: { "1": 4, "2": 6 } });
    json(res, 404, {});
  };
  audit.handler = (_req, res) => json(res, 200, [{ event_id: "e1", type: "VoteRegistered", election_id: 1 }]);
}

before(async () => {
  const cfg: Config = {
    port: 0,
    authUrl: await auth.start(),
    electionUrl: await election.start(),
    voterUrl: await voter.start(),
    voteUrl: await vote.start(),
    resultsUrl: await results.start(),
    auditUrl: await audit.start(),
    adminApiKey: "admin-key",
    serviceApiKey: "service-key",
    corsOrigins: ["http://localhost:3000"],
    upstreamTimeoutMs: 300,
    maxBodyBytes: 1024 * 1024,
  };
  gateway = createGateway(cfg, (m) => logs.push(m));
  await new Promise<void>((r) => gateway.listen(0, "127.0.0.1", r));
  base = `http://127.0.0.1:${(gateway.address() as AddressInfo).port}`;
});

after(async () => {
  gateway.closeAllConnections();
  await new Promise<void>((r) => gateway.close(() => r()));
  await Promise.all([auth, election, voter, vote, results, audit].map((s) => s.stop()));
});

beforeEach(() => {
  for (const s of [auth, election, voter, vote, results, audit]) s.calls = [];
  logs.length = 0;
  defaultHandlers();
});

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function body(res: Response): Promise<any> {
  return res.json();
}

function castVote(body: unknown, token = "jwt-ok"): Promise<Response> {
  return fetch(`${base}/votes`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
  });
}

// ---------------------------------------------------------------------------
// Pruebas
// ---------------------------------------------------------------------------

describe("básico", () => {
  test("GET /health responde ok", async () => {
    const res = await fetch(`${base}/health`);
    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { status: "ok" });
  });

  test("ruta desconocida responde 404 y método equivocado 405", async () => {
    assert.equal((await fetch(`${base}/nada`)).status, 404);
    assert.equal((await fetch(`${base}/votes`)).status, 405);
  });

  test("preflight CORS para el origen del web", async () => {
    const res = await fetch(`${base}/votes`, { method: "OPTIONS", headers: { Origin: "http://localhost:3000" } });
    assert.equal(res.status, 204);
    assert.equal(res.headers.get("access-control-allow-origin"), "http://localhost:3000");
    assert.match(res.headers.get("access-control-allow-headers") ?? "", /Authorization/);
  });

  test("origen no permitido no recibe Access-Control-Allow-Origin", async () => {
    const res = await fetch(`${base}/health`, { headers: { Origin: "http://evil.test" } });
    assert.equal(res.headers.get("access-control-allow-origin"), null);
  });
});

describe("login", () => {
  test("traduce la respuesta de agr-auth-lo al contrato público", async () => {
    const res = await fetch(`${base}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ document: "1001", password: "123456" }),
    });
    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { access_token: "jwt-ok", token_type: "Bearer", expires_in: 3600 });
  });

  test("cuerpo incompleto responde 422 sin llamar a agr-auth-lo", async () => {
    const res = await fetch(`${base}/auth/login`, { method: "POST", body: JSON.stringify({ document: "1001" }) });
    assert.equal(res.status, 422);
    assert.equal(auth.calls.length, 0);
  });

  test("401 de agr-auth-lo se devuelve como 401", async () => {
    auth.handler = (_req, res) => json(res, 401, { detail: "x" });
    const res = await fetch(`${base}/auth/login`, {
      method: "POST",
      body: JSON.stringify({ document: "1001", password: "mala" }),
    });
    assert.equal(res.status, 401);
    assert.equal((await body(res)).detail, "Documento o clave incorrectos");
  });

  test("agr-auth-lo caído responde 502", async () => {
    auth.handler = (_req, res) => {
      res.destroy();
    };
    const res = await fetch(`${base}/auth/login`, {
      method: "POST",
      body: JSON.stringify({ document: "1001", password: "123456" }),
    });
    assert.equal(res.status, 502);
  });
});

describe("saga del voto", () => {
  test("camino feliz: valida, marca, guarda sin datos del votante y responde 201", async () => {
    const res = await castVote({ election_id: 1, candidate_id: 2 });
    assert.equal(res.status, 201);
    assert.deepEqual(await res.json(), { status: "recorded" });

    assert.deepEqual(auth.paths(), ["GET /auth/validate"]);
    assert.deepEqual(election.paths(), ["GET /elections/1"]);
    assert.deepEqual(voter.paths(), ["POST /voters/mark-voted"]);
    assert.deepEqual(JSON.parse(voter.calls[0].body), { voter_id: 7, election_id: 1 });
    assert.equal(voter.calls[0].headers["x-service-key"], "service-key");

    assert.equal(vote.calls.length, 1);
    const sent = vote.calls[0];
    assert.deepEqual(JSON.parse(sent.body), { election_id: 1, candidate_id: 2, candidate_name: "Luis Herrera" });
    assert.equal(sent.headers.authorization, undefined, "el voto no debe llevar el token");
    assert.equal(sent.headers["x-service-key"], "service-key");
    assert.match(String(sent.headers["idempotency-key"]), /^[0-9a-f-]{36}$/);
  });

  test("los logs no contienen el candidato ni el token", async () => {
    await castVote({ election_id: 1, candidate_id: 2 });
    const all = logs.join("\n");
    assert.doesNotMatch(all, /jwt-ok|candidate|Luis/);
    assert.match(all, /"path":"\/votes"/);
  });

  test("sin token responde 401 sin llamar a nadie", async () => {
    const res = await fetch(`${base}/votes`, { method: "POST", body: JSON.stringify({ election_id: 1, candidate_id: 2 }) });
    assert.equal(res.status, 401);
    assert.equal(auth.calls.length + election.calls.length + voter.calls.length + vote.calls.length, 0);
  });

  test("token inválido responde 401 y no marca", async () => {
    const res = await castVote({ election_id: 1, candidate_id: 2 }, "jwt-malo");
    assert.equal(res.status, 401);
    assert.equal(voter.calls.length, 0);
  });

  test("cuerpo inválido responde 422", async () => {
    const res = await castVote({ election_id: "1", candidate_id: 2 });
    assert.equal(res.status, 422);
  });

  test("elección inexistente responde 404 y no marca", async () => {
    const res = await castVote({ election_id: 99, candidate_id: 1 });
    assert.equal(res.status, 404);
    assert.equal(voter.calls.length, 0);
  });

  test("elección cerrada responde 409 y no marca", async () => {
    const res = await castVote({ election_id: 3, candidate_id: 1 });
    assert.equal(res.status, 409);
    assert.equal((await body(res)).detail, "La elección no está abierta");
    assert.equal(voter.calls.length, 0);
  });

  test("candidato de otra elección responde 422 y no marca", async () => {
    const res = await castVote({ election_id: 1, candidate_id: 9 });
    assert.equal(res.status, 422);
    assert.equal(voter.calls.length, 0);
  });

  test("si ya votó en esa elección responde 409 y no guarda", async () => {
    voter.handler = (_req, res) => json(res, 409, { detail: "El votante ya votó" });
    const res = await castVote({ election_id: 1, candidate_id: 2 });
    assert.equal(res.status, 409);
    assert.equal((await body(res)).detail, "Ya votaste en esta elección");
    assert.equal(vote.calls.length, 0);
  });

  test("si el Vote Service responde 500, compensa y responde 500", async () => {
    vote.handler = (_req, res) => json(res, 500, { detail: "mongo caído" });
    const res = await castVote({ election_id: 1, candidate_id: 2 });
    assert.equal(res.status, 500);
    assert.deepEqual(voter.paths(), ["POST /voters/mark-voted", "POST /voters/unmark-voted"]);
    assert.deepEqual(JSON.parse(voter.calls[1].body), { voter_id: 7, election_id: 1 });
    assert.equal(vote.calls.length, 1, "una respuesta explícita no se reintenta");
  });

  test("si el Vote Service no responde a tiempo, reintenta con la misma llave", async () => {
    let n = 0;
    vote.handler = (_req, res) => {
      n += 1;
      if (n === 1) return; // primera vez: nunca responde
      json(res, 201, { status: "recorded" });
    };
    const res = await castVote({ election_id: 1, candidate_id: 2 });
    assert.equal(res.status, 201);
    assert.equal(vote.calls.length, 2);
    assert.equal(vote.calls[0].headers["idempotency-key"], vote.calls[1].headers["idempotency-key"]);
    assert.equal(voter.calls.length, 1, "no hubo compensación");
  });

  test("si el Vote Service tampoco responde al reintento, 502 SIN compensar", async () => {
    vote.handler = () => {
      /* nunca responde */
    };
    const res = await castVote({ election_id: 1, candidate_id: 2 });
    assert.equal(res.status, 502);
    assert.equal(vote.calls.length, 2);
    assert.deepEqual(voter.paths(), ["POST /voters/mark-voted"], "no se desmarca: el voto pudo guardarse");
  });
});

describe("resultados", () => {
  test("une conteos y candidatos: nombres, ceros, porcentajes y orden", async () => {
    const res = await fetch(`${base}/results/1`);
    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), {
      election_id: 1,
      election_name: "Elección de representante estudiantil 2026",
      total_votes: 10,
      results: [
        { candidate_id: 2, candidate_name: "Luis Herrera", votes: 6, percentage: 60 },
        { candidate_id: 1, candidate_name: "Ana Torres", votes: 4, percentage: 40 },
        { candidate_id: 3, candidate_name: "Camila Duarte", votes: 0, percentage: 0 },
      ],
    });
  });

  test("acepta conteos en forma de lista", async () => {
    results.handler = (_req, res) =>
      json(res, 200, { counts: [{ candidate_id: 3, votes: 1 }, { candidate_id: 1, votes: 2 }] });
    const data = await body(await fetch(`${base}/results/1`));
    assert.equal(data.total_votes, 3);
    assert.deepEqual(data.results.map((r: { candidate_id: number }) => r.candidate_id), [1, 3, 2]);
    assert.equal(data.results[0].percentage, 66.67);
  });

  test("elección sin votos todavía: todos en cero", async () => {
    results.handler = (_req, res) => json(res, 404, {});
    const data = await body(await fetch(`${base}/results/1`));
    assert.equal(data.total_votes, 0);
    assert.ok(data.results.every((r: { votes: number }) => r.votes === 0));
  });

  test("elección inexistente responde 404", async () => {
    assert.equal((await fetch(`${base}/results/99`)).status, 404);
  });

  test("Results Service caído responde 502", async () => {
    results.handler = (_req, res) => json(res, 500, {});
    assert.equal((await fetch(`${base}/results/1`)).status, 502);
  });

  test("el SSE se reenvía evento por evento", async () => {
    results.handler = (req, res) => {
      if (!req.path.endsWith("/stream")) return json(res, 404, {});
      res.writeHead(200, { "Content-Type": "text/event-stream" });
      res.write('event: counts\ndata: {"counts":{"1":4}}\n\n');
      setTimeout(() => {
        res.write('event: counts\ndata: {"counts":{"1":5}}\n\n');
        res.end();
      }, 50);
    };
    const res = await fetch(`${base}/results/1/stream`, { headers: { Origin: "http://localhost:3000" } });
    assert.equal(res.status, 200);
    assert.match(res.headers.get("content-type") ?? "", /text\/event-stream/);
    assert.equal(res.headers.get("access-control-allow-origin"), "http://localhost:3000");
    const text = await res.text();
    assert.equal(text, 'event: counts\ndata: {"counts":{"1":4}}\n\nevent: counts\ndata: {"counts":{"1":5}}\n\n');
  });
});

describe("elecciones y administración", () => {
  test("GraphQL se reenvía al Election Service", async () => {
    const res = await fetch(`${base}/graphql`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query: "{ elections { id } }" }),
    });
    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { data: { elections: [{ id: 1 }] } });
    assert.deepEqual(election.paths(), ["POST /graphql"]);
  });

  test("GET /elections conserva los parámetros de consulta", async () => {
    await fetch(`${base}/elections?status=active`);
    assert.deepEqual(election.paths(), ["GET /elections?status=active"]);
  });

  test("admin sin llave responde 401 y no llega al servicio", async () => {
    const res = await fetch(`${base}/admin/elections`, { method: "POST", body: "{}" });
    assert.equal(res.status, 401);
    assert.equal(election.calls.length, 0);
  });

  test("admin con llave quita /admin y reenvía la llave", async () => {
    const res = await fetch(`${base}/admin/elections`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-API-Key": "admin-key" },
      body: JSON.stringify({ name: "Nueva elección" }),
    });
    assert.equal(res.status, 201);
    assert.deepEqual(election.paths(), ["POST /elections"]);
    assert.equal(election.calls[0].headers["x-api-key"], "admin-key");
    assert.deepEqual(JSON.parse(election.calls[0].body), { name: "Nueva elección" });
  });

  test("abrir una elección va a /elections/{id}/activate", async () => {
    await fetch(`${base}/admin/elections/4/activate`, { method: "POST", headers: { "X-API-Key": "admin-key" } });
    assert.deepEqual(election.paths(), ["POST /elections/4/activate"]);
  });

  test("auditoría con llave va a /audit/events con la consulta", async () => {
    const res = await fetch(`${base}/admin/audit?election_id=1&limit=5`, { headers: { "X-API-Key": "admin-key" } });
    assert.equal(res.status, 200);
    assert.deepEqual(audit.paths(), ["GET /audit/events?election_id=1&limit=5"]);
  });
});

describe("Vote Service apagado", () => {
  test("si la conexión es rechazada, compensa y responde 502 (el voto seguro no se guardó)", async () => {
    // Un puerto libre en el que nadie escucha.
    const probe = createServer();
    await new Promise<void>((r) => probe.listen(0, "127.0.0.1", r));
    const deadPort = (probe.address() as AddressInfo).port;
    await new Promise<void>((r) => probe.close(() => r()));

    const cfg: Config = {
      port: 0,
      authUrl: `http://127.0.0.1:${(auth.server.address() as AddressInfo).port}`,
      electionUrl: `http://127.0.0.1:${(election.server.address() as AddressInfo).port}`,
      voterUrl: `http://127.0.0.1:${(voter.server.address() as AddressInfo).port}`,
      voteUrl: `http://127.0.0.1:${deadPort}`,
      resultsUrl: "http://127.0.0.1:1",
      auditUrl: "http://127.0.0.1:1",
      adminApiKey: "admin-key",
      serviceApiKey: "service-key",
      corsOrigins: [],
      upstreamTimeoutMs: 300,
      maxBodyBytes: 1024,
    };
    const gw = createGateway(cfg, () => {});
    await new Promise<void>((r) => gw.listen(0, "127.0.0.1", r));
    try {
      const url = `http://127.0.0.1:${(gw.address() as AddressInfo).port}/votes`;
      const res = await fetch(url, {
        method: "POST",
        headers: { Authorization: "Bearer jwt-ok" },
        body: JSON.stringify({ election_id: 1, candidate_id: 1 }),
      });
      assert.equal(res.status, 502);
      assert.deepEqual(voter.paths(), ["POST /voters/mark-voted", "POST /voters/unmark-voted"]);
    } finally {
      gw.closeAllConnections();
      await new Promise<void>((r) => gw.close(() => r()));
    }
  });
});
