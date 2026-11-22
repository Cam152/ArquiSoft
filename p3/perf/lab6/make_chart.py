"""Lab 6 - Gráficas comparativas de rendimiento: antes y después de las correcciones.

Funcionalidad probada: GET /elections a través del API Gateway (k6, 30 s por nivel).

Uso:
    pip install matplotlib
    python3 make_chart.py

Para cambiar los datos, edita ANTES y DESPUES. Cada fila sale del resumen de k6:
    (vus, http_reqs/s, http_req_duration avg (ms), p(95) (ms), checks OK, checks totales)

Genera performance_comparison.png (y .svg) junto a este archivo.
"""
from pathlib import Path

import matplotlib.pyplot as plt
from matplotlib.ticker import FixedLocator, FuncFormatter, NullLocator

# ---------------------------------------------------------------------------
# DATOS (edita aquí)
# ---------------------------------------------------------------------------

# Antes: Election Service original (1 proceso, pool 5+10, espera 30 s).
ANTES = [
    # vus,  req/s,       avg_ms, p95_ms, ok,    total
    (1,     0.974704,    24.89,  42.57,  30,    30),
    (50,    46.863599,   48.22,  117.27, 1453,  1453),
    (200,   33.242238,   4820,   5160,   46,    1046),
    (500,   79.901131,   5000,   5380,   30,    2530),
    (2000,  837.806659,  1070,   5040,   85,    30098),
]

# Después: conexión liberada antes de serializar, 4 procesos, pool 10+10, espera 3 s.
DESPUES = [
    (1,     0.965646,    34.29,  73.95,  29,    29),
    (50,    47.520185,   35.24,  81.87,  1473,  1473),
    (200,   187.455821,  50.05,  127.4,  5815,  5815),
    (500,   430.7119,    142.26, 259.62, 13357, 13357),
    (2000,  1075.382111, 802.16, 1490,   33971, 33971),
]

# Zona de la rodilla (en usuarios) para cada versión: (desde, hasta).
RODILLA_ANTES = (50, 200)
RODILLA_DESPUES = (500, 2000)

SLEEP_S = 1.0  # sleep(1) del script de k6: cada usuario hace como máximo 1 petición/s

# ---------------------------------------------------------------------------
# Estilo
# ---------------------------------------------------------------------------
C_ANTES, C_DESPUES = "#eb6834", "#2a78d6"
INK, INK2, MUTED, GRID, SURFACE = "#0b0b0b", "#52514e", "#8a8984", "#e6e5e0", "#fcfcfb"

plt.rcParams.update({
    "font.family": "DejaVu Sans",
    "font.size": 10,
    "axes.edgecolor": GRID,
    "axes.labelcolor": INK2,
    "xtick.color": MUTED,
    "ytick.color": MUTED,
    "axes.titlecolor": INK,
    "figure.facecolor": SURFACE,
    "axes.facecolor": SURFACE,
})


def columnas(runs):
    vus = [r[0] for r in runs]
    avg_s = [r[2] / 1000 for r in runs]
    p95_s = [r[3] / 1000 for r in runs]
    ok_s = [r[1] * r[4] / r[5] for r in runs]          # respuestas 200 por segundo
    err = [100 * (1 - r[4] / r[5]) for r in runs]       # % de error
    return vus, avg_s, p95_s, ok_s, err


def preparar_eje(ax, titulo, ylabel, vus):
    ax.set_xscale("log")
    ax.xaxis.set_major_locator(FixedLocator(vus))
    ax.xaxis.set_minor_locator(NullLocator())
    ax.xaxis.set_major_formatter(FuncFormatter(lambda v, _: f"{v:g}"))
    ax.set_xlabel("Usuarios concurrentes (escala log)")
    ax.set_ylabel(ylabel)
    ax.set_title(titulo, loc="left", fontsize=12, fontweight="bold", pad=10)
    ax.grid(axis="y", color=GRID, linewidth=0.8)
    ax.set_axisbelow(True)
    for side in ("top", "right"):
        ax.spines[side].set_visible(False)


def rodillas(ax):
    for (a, b), color in ((RODILLA_ANTES, C_ANTES), (RODILLA_DESPUES, C_DESPUES)):
        ax.axvspan(a, b, color=color, alpha=0.08, linewidth=0)


def serie(ax, x, y, color, label):
    ax.plot(x, y, color=color, linewidth=2, marker="o", markersize=6,
            markeredgecolor=SURFACE, markeredgewidth=1.5, label=label, zorder=3)


def tiempo(y):
    return f"{y:.2f} s" if y >= 1 else f"{y * 1000:.0f} ms"


def main():
    va, avg_a, p95_a, ok_a, err_a = columnas(ANTES)
    vd, avg_d, p95_d, ok_d, err_d = columnas(DESPUES)
    todos_vus = sorted(set(va) | set(vd))

    fig, axes = plt.subplots(1, 3, figsize=(16, 5.4))
    fig.subplots_adjust(left=0.05, right=0.98, top=0.78, bottom=0.2, wspace=0.28)

    # 1. Tiempo de respuesta medio
    ax = axes[0]
    preparar_eje(ax, "Tiempo de respuesta medio", "http_req_duration avg (s)", todos_vus)
    rodillas(ax)
    serie(ax, va, avg_a, C_ANTES, "Antes")
    serie(ax, vd, avg_d, C_DESPUES, "Después")
    for x, y in zip(va, avg_a):
        ax.annotate(tiempo(y), (x, y), textcoords="offset points", xytext=(0, 8),
                    ha="center", fontsize=8, color=INK)
    for x, y in zip(vd, avg_d):
        ax.annotate(tiempo(y), (x, y), textcoords="offset points", xytext=(0, -14),
                    ha="center", fontsize=8, color=INK)
    ax.set_ylim(bottom=-0.3, top=max(avg_a + avg_d) * 1.15)

    # 2. Peticiones exitosas por segundo (+ máximo teórico)
    ax = axes[1]
    preparar_eje(ax, "Peticiones exitosas por segundo", "respuestas 200 / s", todos_vus)
    rodillas(ax)
    ax.plot(todos_vus, [v / SLEEP_S for v in todos_vus], color=MUTED, linewidth=1,
            linestyle="--", label="Máximo teórico (usuarios / 1 s)", zorder=2)
    serie(ax, va, ok_a, C_ANTES, "Antes")
    serie(ax, vd, ok_d, C_DESPUES, "Después")
    for x, y in zip(vd, ok_d):
        ax.annotate(f"{y:.0f}", (x, y), textcoords="offset points", xytext=(-6, 8),
                    ha="right", fontsize=8, color=INK)
    for x, y in zip(va, ok_a):
        ax.annotate(f"{y:.1f}", (x, y), textcoords="offset points", xytext=(6, -12),
                    ha="left", fontsize=8, color=INK)
    ax.set_yscale("symlog", linthresh=10)
    ax.yaxis.set_major_formatter(FuncFormatter(lambda v, _: f"{v:g}"))
    ax.set_ylim(bottom=0)

    # 3. Tasa de error
    ax = axes[2]
    preparar_eje(ax, "Tasa de error", "http_req_failed (%)", todos_vus)
    rodillas(ax)
    serie(ax, va, err_a, C_ANTES, "Antes")
    serie(ax, vd, err_d, C_DESPUES, "Después")
    for x, y in zip(va, err_a):
        ax.annotate(f"{y:.1f}%", (x, y), textcoords="offset points", xytext=(0, 8),
                    ha="center", fontsize=8, color=INK)
    ax.set_ylim(-5, 112)

    # Leyenda única arriba
    handles, labels = axes[1].get_legend_handles_labels()
    orden = [labels.index(l) for l in ("Antes", "Después", "Máximo teórico (usuarios / 1 s)")]
    fig.legend([handles[i] for i in orden], [labels[i] for i in orden], loc="upper left",
               bbox_to_anchor=(0.05, 0.93), ncol=3, frameon=False, fontsize=10)
    fig.text(0.05, 0.97, "Lab 6 · GET /elections vía API Gateway · antes y después de las correcciones en agr-election-lo",
             fontsize=13, fontweight="bold", color=INK, va="top")
    fig.text(0.05, 0.04,
             "Bandas sombreadas: zona de la rodilla (naranja = antes, azul = después). "
             "k6, 30 s por nivel, servicios reiniciados antes de cada corrida.\n"
             "Antes, a 2000 usuarios la latencia media baja porque casi todas las peticiones fallan rápido "
             "(502 del gateway en < 1 s), no porque el sistema mejore.",
             fontsize=8.5, color=MUTED, va="bottom")

    out = Path(__file__).with_name("performance_comparison")
    fig.savefig(out.with_suffix(".png"), dpi=150)
    fig.savefig(out.with_suffix(".svg"))
    print(f"Generado: {out}.png y {out}.svg\n")

    print(f"{'':6}{'ANTES':^38}|{'DESPUÉS':^38}")
    print(f"{'VUs':>6}{'avg':>10}{'p95':>10}{'ok/s':>9}{'err':>9} |{'avg':>10}{'p95':>10}{'ok/s':>9}{'err':>9}")
    for a, d in zip(zip(va, avg_a, p95_a, ok_a, err_a), zip(vd, avg_d, p95_d, ok_d, err_d)):
        print(f"{a[0]:>6}{a[1]*1000:>8.0f}ms{a[2]*1000:>8.0f}ms{a[3]:>9.1f}{a[4]:>8.1f}% |"
              f"{d[1]*1000:>8.0f}ms{d[2]*1000:>8.0f}ms{d[3]:>9.1f}{d[4]:>8.1f}%")


if __name__ == "__main__":
    main()
