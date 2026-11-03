import type { TallyRow } from "@/lib/results";

const percentFormat = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 1 });

const votesLabel = (n: number) => `${n} ${n === 1 ? "voto" : "votos"}`;

/** Barras de resultados. Solo presentación: recibe las filas ya ordenadas. */
export default function ResultBars({ rows }: { rows: TallyRow[] }) {
  const topVotes = Math.max(0, ...rows.map((r) => r.votes));

  return (
    <ol className="results">
      {rows.map((row) => {
        const leading = topVotes > 0 && row.votes === topVotes;
        const width = Math.min(Math.max(row.percentage, 0), 100);
        return (
          <li key={row.candidate_id} className={`result${leading ? " result-leading" : ""}`}>
            <div className="result-head">
              <span className="result-name">
                <span className="candidate-number">{row.number}</span> {row.candidate_name}
                {leading && <span className="leader-tag">Mayor votación</span>}
              </span>
              <span className="result-figures">
                {votesLabel(row.votes)} · {percentFormat.format(row.percentage)} %
              </span>
            </div>
            <div className="bar" aria-hidden="true">
              <div className="bar-fill" style={{ width: `${width}%` }} />
            </div>
          </li>
        );
      })}
    </ol>
  );
}
