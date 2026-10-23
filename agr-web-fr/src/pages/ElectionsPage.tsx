import { Link } from "react-router-dom";
import Notice from "../components/Notice";
import type { LoginRedirectState } from "../components/RequireAuth";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../context/AuthContext";
import { useAsync } from "../hooks/useAsync";
import { fetchElections } from "../services/api";

export default function ElectionsPage() {
  const { session } = useAuth();
  const [state, reload] = useAsync(fetchElections, []);

  return (
    <>
      <header className="page-header">
        <h1>Elecciones</h1>
        <p>Elecciones registradas y sus candidatos.</p>
      </header>

      {state.status === "loading" && <p className="muted">Cargando elecciones…</p>}

      {state.status === "error" && (
        <Notice tone="error" title="No se pudieron cargar las elecciones">
          <p>{state.error.message}</p>
          <div className="actions">
            <button type="button" className="btn" onClick={reload}>
              Reintentar
            </button>
          </div>
        </Notice>
      )}

      {state.status === "success" && state.data.length === 0 && (
        <p className="muted">Todavía no hay elecciones registradas.</p>
      )}

      {state.status === "success" && state.data.length > 0 && (
        <ul className="elections">
          {state.data.map((election) => {
            const votePath = `/elecciones/${election.id}/votar`;
            const loginState: LoginRedirectState = { from: votePath };
            return (
              <li key={election.id} className="election">
                <div className="election-head">
                  <h2>{election.name}</h2>
                  <StatusBadge status={election.status} />
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

                <div className="actions">
                  {election.isOpen && session && (
                    <Link to={votePath} className="btn">
                      Votar
                    </Link>
                  )}
                  {election.isOpen && !session && (
                    <Link to="/ingresar" state={loginState} className="btn">
                      Ingresa para votar
                    </Link>
                  )}
                  {election.status !== "draft" && (
                    <Link to={`/elecciones/${election.id}/resultados`} className="btn btn-secondary">
                      Ver resultados
                    </Link>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </>
  );
}
