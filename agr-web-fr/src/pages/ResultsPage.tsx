import { Link, useParams } from "react-router-dom";
import Notice from "../components/Notice";
import StatusBadge from "../components/StatusBadge";
import { useAsync } from "../hooks/useAsync";
import { fetchElection, fetchResults } from "../services/api";

const percentFormat = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 1 });

const votesLabel = (n: number) => `${n} ${n === 1 ? "voto" : "votos"}`;

export default function ResultsPage() {
  const { id } = useParams();
  const electionId = Number(id);

  // Los resultados vienen del Vote Service. La elección (nombre y estado) es un dato
  // complementario: si el Election Service falla, la página igual muestra el conteo.
  const [state, reload] = useAsync(
    () =>
      Promise.all([
        fetchResults(electionId),
        fetchElection(electionId).catch(() => null),
      ]),
    [electionId],
  );

  const backLink = (
    <Link to="/" className="back-link">
      ← Elecciones
    </Link>
  );

  if (state.status === "loading") {
    return (
      <>
        {backLink}
        <p className="muted">Cargando resultados…</p>
      </>
    );
  }

  if (state.status === "error") {
    return (
      <>
        {backLink}
        <Notice tone="error" title="No se pudieron cargar los resultados">
          <p>{state.error.message}</p>
          {state.error.status !== 404 && (
            <div className="actions">
              <button type="button" className="btn" onClick={reload}>
                Reintentar
              </button>
            </div>
          )}
        </Notice>
      </>
    );
  }

  const [results, election] = state.data;
  const topVotes = Math.max(0, ...results.results.map((r) => r.votes));

  return (
    <>
      {backLink}
      <header className="page-header">
        <div className="election-head">
          <h1>Resultados</h1>
          {election && <StatusBadge status={election.status} />}
        </div>
        {election && <p>{election.name}</p>}
      </header>

      {election?.isOpen && (
        <Notice tone="info">
          <p>La elección sigue abierta: estos resultados son parciales.</p>
        </Notice>
      )}

      <p className="results-total">
        <strong>{results.total_votes}</strong> {results.total_votes === 1 ? "voto" : "votos"} en total
      </p>

      {results.total_votes === 0 && <p className="muted">Aún no hay votos registrados.</p>}

      <ol className="results">
        {results.results.map((result) => {
          const leading = topVotes > 0 && result.votes === topVotes;
          const width = Math.min(Math.max(result.percentage, 0), 100);
          return (
            <li key={result.candidate_id} className={`result${leading ? " result-leading" : ""}`}>
              <div className="result-head">
                <span className="result-name">
                  {result.candidate_name}
                  {leading && <span className="leader-tag">Mayor votación</span>}
                </span>
                <span className="result-figures">
                  {votesLabel(result.votes)} · {percentFormat.format(result.percentage)} %
                </span>
              </div>
              <div className="bar" aria-hidden="true">
                <div className="bar-fill" style={{ width: `${width}%` }} />
              </div>
            </li>
          );
        })}
      </ol>

      <div className="actions">
        <button type="button" className="btn btn-secondary" onClick={reload}>
          Actualizar
        </button>
      </div>
    </>
  );
}
