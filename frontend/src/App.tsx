import { useEffect, useState } from "react";
import { fetchElections, type Election, type ElectionStatus } from "./api";

const STATUS_LABEL: Record<ElectionStatus, string> = {
  draft: "En preparación",
  active: "Abierta",
  closed: "Cerrada",
};

export default function App() {
  const [elections, setElections] = useState<Election[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    setError(null);
    setElections(null);
    fetchElections()
      .then(setElections)
      .catch((e: Error) => setError(e.message));
  };

  useEffect(load, []);

  return (
    <main className="page">
      <header className="page-header">
        <h1>Votaciones</h1>
        <p>Elecciones registradas y sus candidatos.</p>
      </header>

      {error && (
        <div className="notice notice-error" role="alert">
          <p>No se pudieron cargar las elecciones: {error}</p>
          <button type="button" onClick={load}>
            Reintentar
          </button>
        </div>
      )}

      {!error && elections === null && <p className="muted">Cargando elecciones…</p>}

      {elections !== null && elections.length === 0 && (
        <p className="muted">Todavía no hay elecciones registradas.</p>
      )}

      {elections !== null && elections.length > 0 && (
        <ul className="elections">
          {elections.map((election) => (
            <li key={election.id} className="election">
              <div className="election-head">
                <h2>{election.name}</h2>
                <span className={`status status-${election.status}`}>
                  {STATUS_LABEL[election.status]}
                </span>
              </div>

              {election.description && <p className="muted">{election.description}</p>}

              <ol className="candidates">
                {election.candidates.map((candidate) => (
                  <li key={candidate.id}>
                    <span className="candidate-number">{candidate.number}</span>
                    <span>
                      {candidate.name}
                      {candidate.description && (
                        <span className="muted candidate-note">{candidate.description}</span>
                      )}
                    </span>
                  </li>
                ))}
              </ol>

              {election.isOpen && (
                <button
                  type="button"
                  disabled
                  title="Se habilita cuando estén listos el Voter Service y el Vote Service"
                >
                  Votar
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      <footer className="page-footer">
        <p>
          Esta versión solo consulta el Election Service. Faltan el inicio de sesión
          (Voter Service), emitir el voto y ver resultados (Vote Service).
        </p>
      </footer>
    </main>
  );
}
