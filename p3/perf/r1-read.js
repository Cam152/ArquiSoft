// R1 · Lectura: lista de elecciones y resultados, por niveles de usuarios.
//
// Cada iteración hace lo que hace un votante que entra a mirar:
//   GET /elections            (gateway -> agr-election-lo -> agr-elections-db)
//   GET /results/{id}         (gateway compone agr-election-lo + agr-results-lo/Redis)
// y espera SLEEP segundos. Se usa también para R3 (con y sin proxy) y R4 (1 y 3
// réplicas del gateway).
//
//   k6 run -e BASE_URL=http://localhost/api -e LABEL=r1-proxy r1-read.js
//
// Por defecto: niveles 50,200,500,1000,1500 de 30 s. Se cambian con LEVELS y LEVEL_DURATION.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, SLEEP, buildOptions, levelSeconds, parseLevels, writeSummary } from './lib/common.js';

const DEFAULT_LEVELS = '50,200,500,1000,1500';
export const options = buildOptions(DEFAULT_LEVELS, 'read', ['GET /elections', 'GET /results/{id}']);

export function setup() {
  const res = http.get(`${BASE_URL}/elections`);
  if (res.status !== 200) throw new Error(`GET /elections respondió ${res.status}: ${res.body}`);
  const elections = res.json();
  const open = elections.find((e) => e.status === 'active' || e.is_open) || elections[0];
  if (!open) throw new Error('No hay elecciones para consultar resultados');
  return { electionId: open.id };
}

export function read(data) {
  const list = http.get(`${BASE_URL}/elections`, { tags: { name: 'GET /elections' } });
  check(list, { 'elections 200': (r) => r.status === 200 });

  const results = http.get(`${BASE_URL}/results/${data.electionId}`, { tags: { name: 'GET /results/{id}' } });
  check(results, { 'results 200': (r) => r.status === 200 });

  sleep(SLEEP);
}

export function handleSummary(data) {
  return writeSummary(data, parseLevels(DEFAULT_LEVELS), levelSeconds());
}
