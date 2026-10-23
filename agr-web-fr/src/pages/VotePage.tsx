import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import Notice from "../components/Notice";
import type { LoginRedirectState } from "../components/RequireAuth";
import { useAuth } from "../context/AuthContext";
import { useAsync } from "../hooks/useAsync";
import { castVote, fetchElection, toApiError } from "../services/api";

type Step = "choose" | "confirm" | "sending" | "done";

interface VoteError {
  message: string;
  /** Errores de red o del servidor (0, 5xx): tiene sentido volver a intentar. */
  retryable: boolean;
}

export default function VotePage() {
  const { id } = useParams();
  const electionId = Number(id);
  const { session, logout } = useAuth();
  const navigate = useNavigate();

  const [state, reload] = useAsync(() => fetchElection(electionId), [electionId]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [step, setStep] = useState<Step>("choose");
  const [error, setError] = useState<VoteError | null>(null);

  const resultsPath = `/elecciones/${electionId}/resultados`;

  async function submitVote() {
    if (!session || selectedId === null) return;
    setStep("sending");
    setError(null);
    try {
      await castVote(session.token, electionId, selectedId);
      setStep("done");
    } catch (err) {
      const apiError = toApiError(err);
      if (apiError.status === 401) {
        // Token vencido o inválido: al login, y de vuelta a esta pantalla después.
        const loginState: LoginRedirectState = { from: `/elecciones/${electionId}/votar`, expired: true };
        navigate("/ingresar", { replace: true, state: loginState });
        logout();
        return;
      }
      setError({
        message: apiError.message,
        retryable: apiError.status === 0 || apiError.status >= 500,
      });
      setStep("confirm");
    }
  }

  const backLink = (
    <Link to="/" className="back-link">
      ← Elecciones
    </Link>
  );

  if (state.status === "loading") {
    return (
      <>
        {backLink}
        <p className="muted">Cargando elección…</p>
      </>
    );
  }

  if (state.status === "error") {
    return (
      <>
        {backLink}
        <Notice tone="error" title="No se pudo cargar la elección">
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

  const election = state.data;
  const selected = election.candidates.find((c) => c.id === selectedId) ?? null;

  const header = (
    <header className="page-header">
      <h1>Votar</h1>
      <p>{election.name}</p>
    </header>
  );

  if (step === "done") {
    return (
      <>
        {backLink}
        {header}
        <Notice tone="success" title="Voto registrado">
          <p>Tu voto quedó guardado de forma anónima. Gracias por participar.</p>
          <div className="actions">
            <Link to={resultsPath} className="btn">
              Ver resultados
            </Link>
            <Link to="/" className="btn btn-secondary">
              Volver a elecciones
            </Link>
          </div>
        </Notice>
      </>
    );
  }

  // Rechazos definitivos (409, 404, 422): no se muestra el formulario otra vez.
  if (error && !error.retryable) {
    return (
      <>
        {backLink}
        {header}
        <Notice tone="warn" title="No se registró el voto">
          <p>{error.message}</p>
          <div className="actions">
            <Link to={resultsPath} className="btn">
              Ver resultados
            </Link>
            <Link to="/" className="btn btn-secondary">
              Volver a elecciones
            </Link>
          </div>
        </Notice>
      </>
    );
  }

  if (!election.isOpen) {
    return (
      <>
        {backLink}
        {header}
        <Notice tone="info" title="Esta elección no está abierta">
          <p>Solo se puede votar mientras la elección esté abierta.</p>
          <div className="actions">
            <Link to={resultsPath} className="btn btn-secondary">
              Ver resultados
            </Link>
          </div>
        </Notice>
      </>
    );
  }

  if (step === "confirm" || step === "sending") {
    const sending = step === "sending";
    return (
      <>
        {backLink}
        {header}
        <div className="card confirm">
          <p className="muted">Vas a votar por</p>
          <p className="confirm-choice">
            <span className="candidate-number">{selected?.number}</span>
            {selected?.name}
          </p>
          <p className="muted">Una vez enviado, el voto no se puede cambiar.</p>

          {error && (
            <Notice tone="error">
              <p>{error.message}</p>
            </Notice>
          )}

          <div className="actions">
            <button type="button" className="btn" onClick={submitVote} disabled={sending}>
              {sending ? "Enviando voto…" : error ? "Intentar de nuevo" : "Confirmar voto"}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => {
                setError(null);
                setStep("choose");
              }}
              disabled={sending}
            >
              Cambiar
            </button>
          </div>
        </div>
      </>
    );
  }

  return (
    <>
      {backLink}
      {header}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (selected) setStep("confirm");
        }}
      >
        <fieldset className="options-fieldset">
          <legend>Elige un candidato</legend>
          <ul className="options">
            {election.candidates.map((candidate) => (
              <li key={candidate.id}>
                <label className={`option${candidate.id === selectedId ? " option-selected" : ""}`}>
                  <input
                    type="radio"
                    name="candidate"
                    value={candidate.id}
                    checked={candidate.id === selectedId}
                    onChange={() => setSelectedId(candidate.id)}
                  />
                  <span className="candidate-number">{candidate.number}</span>
                  <span>
                    {candidate.name}
                    {candidate.description && (
                      <span className="muted candidate-note">{candidate.description}</span>
                    )}
                  </span>
                </label>
              </li>
            ))}
          </ul>
        </fieldset>
        <div className="actions">
          <button type="submit" className="btn" disabled={!selected}>
            Continuar
          </button>
        </div>
      </form>
    </>
  );
}
