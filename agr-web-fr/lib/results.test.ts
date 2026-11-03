// Pruebas de la lógica pura del web: npm test (no necesitan el gateway).

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { safeReturnPath, parseElectionId } from "./navigation.ts";
import { parseCountsEvent, tally, votesFromResults } from "./results.ts";

const CANDIDATES = [
  { id: 1, number: 1, name: "Ana Torres" },
  { id: 2, number: 2, name: "Luis Herrera" },
  { id: 3, number: 3, name: "Camila Duarte" },
];

describe("tally", () => {
  it("calcula como la composición del gateway: total, porcentajes y orden", () => {
    const { total, rows } = tally(CANDIDATES, { 1: 4, 2: 6 });
    assert.equal(total, 10);
    assert.deepEqual(
      rows.map((r) => [r.candidate_id, r.votes, r.percentage]),
      [
        [2, 6, 60],
        [1, 4, 40],
        [3, 0, 0],
      ],
    );
  });

  it("desempata por número de tarjetón y no divide por cero", () => {
    const { total, rows } = tally(CANDIDATES, {});
    assert.equal(total, 0);
    assert.deepEqual(rows.map((r) => r.number), [1, 2, 3]);
    assert.ok(rows.every((r) => r.percentage === 0));
  });

  it("redondea a dos decimales", () => {
    const { rows } = tally(CANDIDATES, { 1: 1, 2: 1, 3: 1 });
    assert.deepEqual(rows.map((r) => r.percentage), [33.33, 33.33, 33.33]);
  });

  it("ignora conteos de candidatos que la página no conoce", () => {
    const { total } = tally(CANDIDATES, { 1: 2, 99: 5 });
    assert.equal(total, 2);
  });
});

describe("parseCountsEvent", () => {
  it("lee el evento del contrato", () => {
    const raw = '{"election_id": 1, "total_votes": 11, "counts": {"1": 4, "2": 7}}';
    assert.deepEqual(parseCountsEvent(raw, 1), { 1: 4, 2: 7 });
  });

  it("descarta eventos de otra elección o mal formados", () => {
    assert.equal(parseCountsEvent('{"election_id": 2, "counts": {}}', 1), null);
    assert.equal(parseCountsEvent("keep-alive", 1), null);
    assert.equal(parseCountsEvent('{"election_id": 1}', 1), null);
    assert.deepEqual(parseCountsEvent('{"election_id": 1, "counts": {"x": 1, "2": -3, "3": 2}}', 1), { 3: 2 });
  });
});

describe("votesFromResults", () => {
  it("convierte la respuesta de GET /results/{id}", () => {
    const votes = votesFromResults([
      { candidate_id: 2, candidate_name: "Luis Herrera", votes: 6, percentage: 60 },
      { candidate_id: 1, candidate_name: "Ana Torres", votes: 4, percentage: 40 },
    ]);
    assert.deepEqual(votes, { 1: 4, 2: 6 });
  });
});

describe("navegación", () => {
  it("solo vuelve a rutas internas después del login", () => {
    assert.equal(safeReturnPath("/elecciones/1/votar"), "/elecciones/1/votar");
    assert.equal(safeReturnPath(["/elecciones/2/votar", "/x"]), "/elecciones/2/votar");
    for (const bad of [undefined, "", "https://evil.example", "//evil.example", "/\\evil.example", "elecciones"]) {
      assert.equal(safeReturnPath(bad), "/", String(bad));
    }
  });

  it("valida el id de la elección", () => {
    assert.equal(parseElectionId("1"), 1);
    for (const bad of ["0", "-1", "1.5", "abc", "99999999999", "2147483648"]) {
      assert.equal(parseElectionId(bad), null, bad);
    }
  });
});
