"""p3/perf · gráficas y tabla resumen de R1 a R4.

Lee results/<corrida>/summary.json (k6) y docker-stats-summary.txt y genera en
p3/perf/charts/:

    r1_lectura.png              R1: latencia p50/p95/p99 y peticiones por segundo (con proxy)
    r2_escritura.png            R2: latencia p95 de login y voto, votos por segundo y errores
    r3_costo_seguridad.png      R3: línea base (sin proxy) contra proxy, para R1 y R2
    r4_escalabilidad.png        R4: R1 con 1 y con 3 réplicas del gateway
    recursos_cpu.png            CPU promedio por contenedor en cada corrida
y results/RESUMEN.md con todas las tablas.

    pip install matplotlib
    python3 p3/perf/make_charts.py
"""
import json
from pathlib import Path

import matplotlib.pyplot as plt
from matplotlib.ticker import FixedLocator, FuncFormatter, NullLocator

HERE = Path(__file__).resolve().parent
RESULTS = HERE / "results"
CHARTS = HERE / "charts"

# Paleta de referencia (validada): azul y naranja como series 1 y 2.
BLUE, ORANGE, AQUA = "#2a78d6", "#eb6834", "#1baf7a"
INK, INK2, MUTED, GRID, SURFACE = "#0b0b0b", "#52514e", "#8a8984", "#e6e5e0", "#fcfcfb"

plt.rcParams.update({
    "font.family": "DejaVu Sans", "font.size": 10,
    "axes.edgecolor": GRID, "axes.labelcolor": INK2,
    "xtick.color": MUTED, "ytick.color": MUTED, "axes.titlecolor": INK,
    "figure.facecolor": SURFACE, "axes.facecolor": SURFACE,
})


def load(label):
    p = RESULTS / label / "summary.json"
    if not p.exists():
        return None
    data = json.loads(p.read_text())
    return {"label": label, "sleep": data.get("sleep_s", 1), "base_url": data["base_url"], "levels": data["levels"]}


def load_stats(label):
    p = RESULTS / label / "docker-stats-summary.txt"
    if not p.exists():
        return {}
    out = {}
    for line in p.read_text().splitlines()[1:]:
        parts = line.split()
        if len(parts) == 4:
            name = parts[0].replace("arquisoft-", "")
            out[name] = (float(parts[1]), float(parts[2]), float(parts[3]))
    return out


def col(run, key):
    return [lv[key] for lv in run["levels"]]


def ok_rps(run):
    # Peticiones exitosas por segundo
    return [lv["rps"] * (1 - lv["error_rate"]) for lv in run["levels"]]


def ep(run, name, key):
    return [lv["endpoints"].get(name, {}).get(key, float("nan")) for lv in run["levels"]]


def axis(ax, title, ylabel, vus):
    ax.set_xscale("log")
    ax.xaxis.set_major_locator(FixedLocator(vus))
    ax.xaxis.set_minor_locator(NullLocator())
    ax.xaxis.set_major_formatter(FuncFormatter(lambda v, _: f"{v:g}"))
    ax.set_xlabel("Usuarios virtuales (escala log)")
    ax.set_ylabel(ylabel)
    ax.set_title(title, loc="left", fontsize=11.5, fontweight="bold", pad=10)
    ax.grid(axis="y", color=GRID, linewidth=0.8)
    ax.set_axisbelow(True)
    for side in ("top", "right"):
        ax.spines[side].set_visible(False)


def line(ax, x, y, color, label, style="-"):
    ax.plot(x, y, color=color, linewidth=2, linestyle=style, marker="o", markersize=6,
            markeredgecolor=SURFACE, markeredgewidth=1.5, label=label, zorder=3)


def end_label(ax, x, y, text, dy=0):
    ax.annotate(text, (x[-1], y[-1]), textcoords="offset points", xytext=(6, dy),
                ha="left", va="center", fontsize=8.5, color=INK2)


def rank_dy(values):
    """Desplazamiento vertical de cada etiqueta final según su orden: la mayor arriba."""
    order = sorted(range(len(values)), key=lambda i: values[i])
    dy = [0] * len(values)
    for rank, i in enumerate(order):
        dy[i] = (rank - (len(values) - 1) / 2) * 11
    return dy


def fmt_ms(v):
    return f"{v / 1000:.2f} s" if v >= 1000 else f"{v:.0f} ms"


def header(fig, title, note):
    fig.text(0.045, 0.97, title, fontsize=13, fontweight="bold", color=INK, va="top")
    fig.text(0.045, 0.025, note, fontsize=8.5, color=MUTED, va="bottom")


def legend(fig, axes_with_handles, y=0.915):
    handles, labels = axes_with_handles.get_legend_handles_labels()
    fig.legend(handles, labels, loc="upper left", bbox_to_anchor=(0.045, y), ncol=len(labels),
               frameon=False, fontsize=10)


def compare(runs, names, colors, title, note, out, panels):
    """Fila de paneles comparando varias corridas con los mismos niveles."""
    fig, axes = plt.subplots(len(panels), 3, figsize=(16, 4.9 * len(panels)), squeeze=False)
    fig.subplots_adjust(left=0.05, right=0.95, top=1 - 0.2 / len(panels), bottom=0.16 / len(panels) + 0.04,
                        wspace=0.3, hspace=0.55)
    for row, (row_title, row_runs) in enumerate(panels):
        rr = [r for r in row_runs if r]
        if not rr:
            continue
        vus = sorted({v for r in rr for v in col(r, "vus")})
        a = axes[row][0]
        axis(a, f"{row_title} · latencia p95", "http_req_duration p95 (ms)", vus)
        present = [(r, n, c) for r, n, c in zip(row_runs, names, colors) if r]
        dys = rank_dy([(col(r, "p95_ms"))[-1] for r, _, _ in present])
        for (r, n, c), dy in zip(present, dys):
            line(a, col(r, "vus"), col(r, "p95_ms"), c, n)
            end_label(a, col(r, "vus"), col(r, "p95_ms"), fmt_ms(col(r, "p95_ms")[-1]), dy=dy)
        a.set_ylim(bottom=0)
        a = axes[row][1]
        axis(a, f"{row_title} · peticiones exitosas por segundo", "respuestas OK / s", vus)
        present = [(r, n, c) for r, n, c in zip(row_runs, names, colors) if r]
        dys = rank_dy([(ok_rps(r))[-1] for r, _, _ in present])
        for (r, n, c), dy in zip(present, dys):
            line(a, col(r, "vus"), ok_rps(r), c, n)
            end_label(a, col(r, "vus"), ok_rps(r), f"{ok_rps(r)[-1]:.0f}/s", dy=dy)
        a.set_ylim(bottom=0)
        a = axes[row][2]
        axis(a, f"{row_title} · tasa de error", "http_req_failed (%)", vus)
        for r, n, c in zip(row_runs, names, colors):
            if r:
                line(a, col(r, "vus"), [100 * e for e in col(r, "error_rate")], c, n)
        a.set_ylim(-3, 103)
    legend(fig, axes[0][0], y=1 - 0.085 / len(panels))
    header(fig, title, note)
    fig.savefig(CHARTS / out, dpi=150)
    plt.close(fig)


def r1_chart(run):
    vus = col(run, "vus")
    fig, axes = plt.subplots(1, 2, figsize=(13, 5.2))
    fig.subplots_adjust(left=0.06, right=0.93, top=0.84, bottom=0.2, wspace=0.28)
    a = axes[0]
    axis(a, "Latencia por percentil", "http_req_duration (ms)", vus)
    for key, c, n in (("p50_ms", AQUA, "p50"), ("p95_ms", BLUE, "p95"), ("p99_ms", ORANGE, "p99")):
        line(a, vus, col(run, key), c, n)
        end_label(a, vus, col(run, key), f"{n} {fmt_ms(col(run, key)[-1])}")
    a.set_ylim(bottom=0)
    a = axes[1]
    axis(a, "Peticiones exitosas por segundo", "respuestas OK / s", vus)
    teo = [2 * v / run["sleep"] for v in vus]
    a.plot(vus, teo, color=MUTED, linewidth=1, linestyle="--", label="Máximo teórico (2 peticiones / usuario / s)")
    line(a, vus, ok_rps(run), BLUE, "Medido")
    for x, y in zip(vus, ok_rps(run)):
        a.annotate(f"{y:.0f}", (x, y), textcoords="offset points", xytext=(-4, 9), ha="right", fontsize=8, color=INK)
    a.set_ylim(bottom=0)
    for a in axes:
        a.legend(loc="upper left", frameon=False, fontsize=9)
    header(fig, "R1 · Lectura: GET /elections + GET /results/{id} a través del proxy (1 réplica del gateway)",
           "k6 en el mismo equipo, niveles de 30 s con usuarios constantes y sleep(1) por iteración.")
    fig.savefig(CHARTS / "r1_lectura.png", dpi=150)
    plt.close(fig)


def r2_chart(run):
    vus = col(run, "vus")
    fig, axes = plt.subplots(1, 3, figsize=(16, 5.2))
    fig.subplots_adjust(left=0.05, right=0.95, top=0.76, bottom=0.2, wspace=0.3)
    a = axes[0]
    axis(a, "Latencia p95 por paso", "http_req_duration p95 (ms)", vus)
    for name, c in (("POST /auth/login", BLUE), ("POST /votes", ORANGE)):
        line(a, vus, ep(run, name, "p95_ms"), c, name)
        end_label(a, vus, ep(run, name, "p95_ms"), fmt_ms(ep(run, name, "p95_ms")[-1]))
    a.set_ylim(bottom=0)
    a = axes[1]
    axis(a, "Votos registrados por segundo", "votos 201 / s", vus)
    # Votos OK/s: peticiones a POST /votes del nivel por su tasa de éxito, sobre la duración del nivel
    # (reqs / rps). Las iteraciones cuyo login falló no llegan a votar.
    votes = []
    for lv in run["levels"]:
        dur = lv["reqs"] / lv["rps"] if lv["rps"] else 1
        v = lv["endpoints"].get("POST /votes", {})
        login_ok = 1 - lv["endpoints"].get("POST /auth/login", {}).get("error_rate", 0)
        votes.append(lv["iterations"] * login_ok * (1 - v.get("error_rate", 1)) / dur)
    a.plot(vus, [v / run["sleep"] for v in vus], color=MUTED, linewidth=1, linestyle="--",
           label="Máximo teórico (1 voto / usuario / s)")
    line(a, vus, votes, BLUE, "Votos registrados")
    for x, y in zip(vus, votes):
        a.annotate(f"{y:.0f}", (x, y), textcoords="offset points", xytext=(-4, 9), ha="right", fontsize=8, color=INK)
    a.set_ylim(bottom=0)
    a = axes[2]
    axis(a, "Tasa de error por paso", "http_req_failed (%)", vus)
    for name, c in (("POST /auth/login", BLUE), ("POST /votes", ORANGE)):
        line(a, vus, [100 * e for e in ep(run, name, "error_rate")], c, name)
    a.set_ylim(-3, 103)
    h1, l1 = axes[0].get_legend_handles_labels()
    h2, l2 = axes[1].get_legend_handles_labels()
    fig.legend(h1 + h2[:1], l1 + l2[:1], loc="upper left", bbox_to_anchor=(0.05, 0.9), ncol=3, frameon=False)
    header(fig, "R2 · Escritura: login + voto (saga completa) a través del proxy",
           "Cada iteración es un votante distinto (seed-voters.sql). Límite de login de S4 subido durante la prueba "
           "(docker-compose.perf.yml). Niveles de 30 s, sleep(1).")
    fig.savefig(CHARTS / "r2_escritura.png", dpi=150)
    plt.close(fig)


def cpu_chart(labels):
    stats = {l: load_stats(l) for l in labels}
    stats = {l: s for l, s in stats.items() if s}
    if not stats:
        return
    # Los 7 contenedores con más CPU en alguna corrida
    names = sorted({n for s in stats.values() for n in s},
                   key=lambda n: -max(s.get(n, (0, 0, 0))[0] for s in stats.values()))[:7]
    fig, axes = plt.subplots(1, len(stats), figsize=(4.2 * len(stats), 4.8), sharex=True)
    if len(stats) == 1:
        axes = [axes]
    fig.subplots_adjust(left=0.13, right=0.98, top=0.8, bottom=0.22, wspace=0.08)
    xmax = max(s.get(n, (0, 0, 0))[0] for s in stats.values() for n in names) * 1.25
    for a, (label, s) in zip(axes, stats.items()):
        vals = [s.get(n, (0, 0, 0))[0] for n in names]
        a.barh(range(len(names)), vals, color=BLUE, height=0.6)
        for i, v in enumerate(vals):
            a.text(v + xmax * 0.01, i, f"{v:.0f}%", va="center", fontsize=8.5, color=INK2)
        a.set_yticks(range(len(names)))
        a.set_yticklabels(names if a is axes[0] else [])
        a.invert_yaxis()
        a.set_xlim(0, xmax)
        a.set_title(label, loc="left", fontsize=11, fontweight="bold")
        a.grid(axis="x", color=GRID, linewidth=0.8)
        a.set_axisbelow(True)
        for side in ("top", "right"):
            a.spines[side].set_visible(False)
        a.set_xlabel("CPU promedio (%; 100 % = 1 núcleo)")
    header(fig, "Uso de CPU por contenedor durante cada corrida (docker stats)",
           "Promedio de todas las muestras de la corrida (incluye las pausas entre niveles).")
    fig.savefig(CHARTS / "recursos_cpu.png", dpi=150)
    plt.close(fig)


def table(run):
    rows = ["| VUs | req/s | p50 | p95 | p99 | error |", "|---:|---:|---:|---:|---:|---:|"]
    for lv in run["levels"]:
        rows.append(f"| {lv['vus']} | {lv['rps']:.1f} | {fmt_ms(lv['p50_ms'])} | {fmt_ms(lv['p95_ms'])} | "
                    f"{fmt_ms(lv['p99_ms'])} | {100 * lv['error_rate']:.2f} % |")
    return "\n".join(rows)


def ep_table(run):
    rows = ["| VUs | Endpoint | p50 | p95 | p99 | error |", "|---:|---|---:|---:|---:|---:|"]
    for lv in run["levels"]:
        for name, e in sorted(lv["endpoints"].items()):
            rows.append(f"| {lv['vus']} | `{name}` | {fmt_ms(e['p50_ms'])} | {fmt_ms(e['p95_ms'])} | "
                        f"{fmt_ms(e['p99_ms'])} | {100 * e['error_rate']:.2f} % |")
    return "\n".join(rows)


def main():
    CHARTS.mkdir(exist_ok=True)
    runs = {l: load(l) for l in ("r1-base", "r1-proxy", "r2-base", "r2-proxy", "r4-3x",
                                 "r1-proxy-sin-keepalive", "r2-proxy-sin-keepalive", "r4-3x-sin-keepalive")}
    if runs["r1-proxy"]:
        r1_chart(runs["r1-proxy"])
    if runs["r2-proxy"]:
        r2_chart(runs["r2-proxy"])
    compare(runs, ["Sin proxy (línea base)", "Con proxy (keepalive)", "Con proxy, sin keepalive (primera versión)"],
            [ORANGE, BLUE, AQUA],
            "R3 · Costo de la seguridad en el borde: gateway directo contra gateway detrás de agr-proxy-rp",
            "Línea base: p3 con la segmentación de S3 y el gateway publicado en :8080 (docker-compose.baseline.yml). "
            "Con proxy: /api en agr-proxy-rp (encabezados, 1 MB, registro; límite de login subido para R2).",
            "r3_costo_seguridad.png",
            [("R1 lectura", [runs["r1-base"], runs["r1-proxy"], runs["r1-proxy-sin-keepalive"]]),
             ("R2 escritura", [runs["r2-base"], runs["r2-proxy"], None])])
    compare(runs, ["1 réplica del gateway", "3 réplicas del gateway"], [ORANGE, BLUE],
            "R4 · Escalabilidad horizontal: R1 con 1 y con 3 réplicas de agr-gateway-or detrás del proxy",
            "docker compose up --scale agr-gateway-or=3. El proxy reparte en round robin: upstream agr_gateway con "
            "\"server agr-gateway-or:8080 resolve\" (DNS de Docker) y keepalive 64.",
            "r4_escalabilidad.png",
            [("R1 lectura", [runs["r1-proxy"], runs["r4-3x"]])])
    cpu_chart(["r1-base", "r1-proxy", "r4-3x", "r2-base", "r2-proxy"])

    md = ["# Resultados R1 a R4 (generado por make_charts.py)", ""]
    titles = {"r1-base": "R1 · Lectura sin proxy (línea base)", "r1-proxy": "R1 · Lectura con proxy (= R4, 1 réplica)",
              "r4-3x": "R4 · Lectura con proxy y 3 réplicas del gateway", "r2-base": "R2 · Escritura sin proxy (línea base)",
              "r2-proxy": "R2 · Escritura con proxy",
              "r1-proxy-sin-keepalive": "R1 · Lectura con proxy, primera versión (sin keepalive hacia el gateway)",
              "r2-proxy-sin-keepalive": "R2 · Escritura con proxy, primera versión (sin keepalive)",
              "r4-3x-sin-keepalive": "R4 · 3 réplicas, primera versión del proxy (sin keepalive)"}
    for label, title in titles.items():
        r = runs[label]
        if not r:
            continue
        md += [f"## {title}", "", f"`{r['base_url']}` · resumen en `results/{label}/`", "", table(r), "",
               ep_table(r), ""]
        st = (RESULTS / label / "docker-stats-summary.txt")
        if st.exists():
            md += ["```", st.read_text().rstrip(), "```", ""]
    (RESULTS / "RESUMEN.md").write_text("\n".join(md))
    print(f"Gráficas en {CHARTS}, tablas en {RESULTS / 'RESUMEN.md'}")


if __name__ == "__main__":
    main()
