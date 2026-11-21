// Utilidades compartidas por R1 y R2 (p3/perf).
//
// Cada prueba corre una serie de NIVELES de usuarios virtuales, uno tras otro,
// cada uno como un escenario constant-vus de LEVEL_DURATION. Así el resumen de
// k6 trae las métricas de cada nivel por separado (submétricas por escenario),
// igual que las corridas por nivel del Lab 6 pero en una sola ejecución.
//
// Variables de entorno (k6 run -e NOMBRE=valor):
//   BASE_URL        URL base de la API. Con el proxy: http://localhost/api
//                   (https://localhost/api con S1). Línea base: http://localhost:8080
//   LEVELS          niveles de usuarios, separados por coma (cada prueba trae los suyos)
//   LEVEL_DURATION  duración de cada nivel (por defecto 30s)
//   PAUSE           pausa entre niveles para que el sistema se vacíe (por defecto 5s)
//   SLEEP           pausa de cada usuario entre iteraciones, en segundos (por defecto 1)
//   LABEL           nombre de la corrida; va al archivo de resumen

export const BASE_URL = (__ENV.BASE_URL || 'http://localhost/api').replace(/\/+$/, '');
export const SLEEP = Number(__ENV.SLEEP || 1);
export const LABEL = __ENV.LABEL || 'run';

function seconds(d) {
  const m = /^(\d+)(ms|s|m)$/.exec(d);
  if (!m) throw new Error(`Duración no válida: ${d}`);
  const n = Number(m[1]);
  return m[2] === 'ms' ? n / 1000 : m[2] === 'm' ? n * 60 : n;
}

export function parseLevels(defaultLevels) {
  return (__ENV.LEVELS || defaultLevels).split(',').map((v) => Number(v.trim())).filter((v) => v > 0);
}

// Escenarios secuenciales (uno por nivel) y umbrales "siempre verdaderos" que
// obligan a k6 a calcular y publicar las submétricas de cada escenario.
export function buildOptions(defaultLevels, exec, endpoints = []) {
  const levels = parseLevels(defaultLevels);
  const dur = __ENV.LEVEL_DURATION || '30s';
  const pause = seconds(__ENV.PAUSE || '5s');
  const scenarios = {};
  const thresholds = {};
  let start = 0;
  for (const vus of levels) {
    const name = `vus_${vus}`;
    scenarios[name] = {
      executor: 'constant-vus',
      exec,
      vus,
      duration: dur,
      startTime: `${start}s`,
      gracefulStop: '10s',
      tags: { level: String(vus) },
    };
    for (const metric of ['http_req_duration', 'http_reqs', 'http_req_failed', 'checks', 'iterations']) {
      const agg = { http_req_duration: 'max>=0', http_req_failed: 'rate>=0', checks: 'rate>=0' }[metric] || 'count>=0';
      thresholds[`${metric}{scenario:${name}}`] = [agg];
    }
    for (const ep of endpoints) {
      thresholds[`http_req_duration{scenario:${name},name:${ep}}`] = ['max>=0'];
      thresholds[`http_req_failed{scenario:${name},name:${ep}}`] = ['rate>=0'];
    }
    start += seconds(dur) + 10 + pause; // gracefulStop + pausa
  }
  thresholds['http_req_failed{scenario:none}'] = ['rate>=0'];
  return {
    scenarios,
    thresholds,
    summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
    // Con S1 el certificado es autofirmado (localhost).
    insecureSkipTLSVerify: true,
    discardResponseBodies: false,
    noConnectionReuse: false,
    setupTimeout: '60s',
    tags: { label: LABEL },
  };
}

// Guarda el resumen completo en JSON (lo procesa make_charts.py) y una tabla
// corta por nivel en texto, además del resumen normal de k6 en consola.
export function writeSummary(data, levels, durationSec) {
  const rows = [];
  for (const vus of levels) {
    const s = `scenario:vus_${vus}`;
    const m = (name) => data.metrics[`${name}{${s}}`];
    const dur = m('http_req_duration');
    const reqs = m('http_reqs');
    const failed = m('http_req_failed');
    const checks = m('checks');
    const iters = m('iterations');
    if (!dur || !reqs) continue;
    rows.push({
      vus,
      reqs: reqs.values.count,
      rps: reqs.values.count / durationSec,
      iterations: iters ? iters.values.count : 0,
      avg_ms: dur.values.avg,
      p50_ms: dur.values.med,
      p95_ms: dur.values['p(95)'],
      p99_ms: dur.values['p(99)'],
      max_ms: dur.values.max,
      error_rate: failed ? failed.values.rate : 0,
      checks_ok: checks ? checks.values.passes : 0,
      checks_total: checks ? checks.values.passes + checks.values.fails : 0,
      endpoints: Object.fromEntries(
        Object.keys(data.metrics)
          .filter((k) => k.startsWith(`http_req_duration{${s},name:`))
          .map((k) => {
            const ep = k.slice(`http_req_duration{${s},name:`.length, -1);
            const d = data.metrics[k].values;
            const f = data.metrics[`http_req_failed{${s},name:${ep}}`];
            return [ep, { avg_ms: d.avg, p50_ms: d.med, p95_ms: d['p(95)'], p99_ms: d['p(99)'], error_rate: f ? f.values.rate : 0 }];
          }),
      ),
    });
  }
  const lines = [
    `# ${LABEL} · ${BASE_URL}`,
    'VUs      reqs     req/s    p50 ms    p95 ms    p99 ms   error %   checks',
    ...rows.map((r) =>
      [
        String(r.vus).padStart(4),
        String(r.reqs).padStart(9),
        r.rps.toFixed(1).padStart(9),
        r.p50_ms.toFixed(1).padStart(9),
        r.p95_ms.toFixed(1).padStart(9),
        r.p99_ms.toFixed(1).padStart(9),
        (100 * r.error_rate).toFixed(2).padStart(9),
        `   ${r.checks_ok}/${r.checks_total}`,
      ].join(' '),
    ),
  ];
  for (const r of rows) {
    for (const [ep, e] of Object.entries(r.endpoints)) {
      lines.push(`     ${String(r.vus).padStart(4)} VUs · ${ep.padEnd(20)} p50 ${e.p50_ms.toFixed(1)} ms · p95 ${e.p95_ms.toFixed(1)} ms · p99 ${e.p99_ms.toFixed(1)} ms · error ${(100 * e.error_rate).toFixed(2)} %`);
    }
  }
  const out = __ENV.OUT_DIR || '/out';
  return {
    stdout: lines.join('\n') + '\n',
    [`${out}/summary.json`]: JSON.stringify({ label: LABEL, base_url: BASE_URL, sleep_s: SLEEP, levels: rows, raw: data }, null, 2),
    [`${out}/summary.txt`]: lines.join('\n') + '\n',
  };
}

export function levelSeconds() {
  return seconds(__ENV.LEVEL_DURATION || '30s');
}
