// R2 · Escritura: login y voto (la saga completa), por niveles hasta el quiebre.
//
// Cada iteración es un votante distinto que:
//   POST /auth/login   (agr-auth-lo: BCrypt + JWT)
//   POST /votes        (saga en el gateway: auth valida el JWT, election revisa
//                       la elección, voter marca la participación, vote guarda el
//                       voto y lo publica en el broker)
//
// Los votantes salen de seed-voters.sql: documentos PREFIX + número de 6 dígitos
// (por defecto 9000001, 9000002, ...) con la clave 123456. Cada votante vota una
// sola vez por elección, así que el número de votante nunca se repite: lo arma
// el id del usuario virtual (único en toda la prueba) y su número de iteración.
//
//   k6 run -e BASE_URL=http://localhost/api -e LABEL=r2-proxy r2-write.js
//
// Antes: subir el límite de login de S4 (LOGIN_RATE/LOGIN_BURST, ver README), si
// no la prueba mide el límite (429) y no el sistema.

import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { BASE_URL, SLEEP, buildOptions, levelSeconds, parseLevels, writeSummary } from './lib/common.js';

const DEFAULT_LEVELS = '10,25,50,100,200';
export const options = buildOptions(DEFAULT_LEVELS, 'vote', ['POST /auth/login', 'POST /votes']);

const PREFIX = __ENV.VOTER_PREFIX || '9';
const PASSWORD = __ENV.VOTER_PASSWORD || '123456';
// Iteraciones máximas que puede hacer un usuario virtual en toda la prueba.
// Votantes necesarios = (máximo de VUs) × ITER_CAP; seed-voters.sql crea 40000.
const ITER_CAP = Number(__ENV.ITER_CAP || 200);
const VOTERS = Number(__ENV.VOTERS || 40000);
const JSON_HEADERS = { 'Content-Type': 'application/json' };

export function setup() {
  const res = http.get(`${BASE_URL}/elections`);
  if (res.status !== 200) throw new Error(`GET /elections respondió ${res.status}: ${res.body}`);
  const open = res.json().find((e) => e.status === 'active' || e.is_open);
  if (!open) throw new Error('No hay una elección abierta para votar');
  const detail = http.get(`${BASE_URL}/elections/${open.id}`).json();
  return { electionId: open.id, candidates: detail.candidates.map((c) => c.id) };
}

export function vote(data) {
  const n = (exec.vu.idInTest - 1) * ITER_CAP + exec.vu.iterationInInstance + 1;
  if (exec.vu.iterationInInstance >= ITER_CAP || n > VOTERS) {
    exec.test.abort(`Se acabaron los votantes sembrados (n=${n}); suba VOTERS/ITER_CAP y siembre más`);
  }
  const document = `${PREFIX}${String(n).padStart(6, '0')}`;

  const login = http.post(`${BASE_URL}/auth/login`, JSON.stringify({ document, password: PASSWORD }), {
    headers: JSON_HEADERS,
    tags: { name: 'POST /auth/login' },
  });
  const okLogin = check(login, { 'login 200': (r) => r.status === 200 });
  if (!okLogin) {
    sleep(SLEEP);
    return;
  }

  const candidate = data.candidates[n % data.candidates.length];
  const ballot = http.post(
    `${BASE_URL}/votes`,
    JSON.stringify({ election_id: data.electionId, candidate_id: candidate }),
    {
      headers: { ...JSON_HEADERS, Authorization: `Bearer ${login.json('access_token')}` },
      tags: { name: 'POST /votes' },
    },
  );
  check(ballot, { 'vote 201': (r) => r.status === 201 });

  sleep(SLEEP);
}

export function handleSummary(data) {
  return writeSummary(data, parseLevels(DEFAULT_LEVELS), levelSeconds());
}
