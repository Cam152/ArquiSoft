import { useState, type FormEvent } from "react";
import { Navigate, useLocation } from "react-router-dom";
import Notice from "../components/Notice";
import type { LoginRedirectState } from "../components/RequireAuth";
import { useAuth } from "../context/AuthContext";
import { toApiError } from "../services/api";

export default function LoginPage() {
  const { session, login } = useAuth();
  const location = useLocation();
  const { from = "/", expired = false } = (location.state ?? {}) as LoginRedirectState;

  const [documentNumber, setDocumentNumber] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Con sesión activa (incluido justo después de un login exitoso) se vuelve a donde se venía.
  if (session) return <Navigate to={from} replace />;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const document = documentNumber.trim();
    if (!document || !password) {
      setError("Ingresa tu documento y tu clave.");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await login(document, password);
    } catch (err) {
      setError(toApiError(err).message);
      setSubmitting(false);
    }
  }

  return (
    <section className="narrow">
      <header className="page-header">
        <h1>Ingresar</h1>
        <p>Usa tu número de documento y la clave que te asignaron.</p>
      </header>

      {expired && !error && (
        <Notice tone="warn">
          <p>Tu sesión expiró. Ingresa de nuevo para continuar.</p>
        </Notice>
      )}

      {error && (
        <Notice tone="error">
          <p>{error}</p>
        </Notice>
      )}

      <form className="card form" onSubmit={handleSubmit} noValidate>
        <div className="field">
          <label htmlFor="document">Número de documento</label>
          <input
            id="document"
            name="document"
            inputMode="numeric"
            autoComplete="username"
            value={documentNumber}
            onChange={(e) => setDocumentNumber(e.target.value)}
            disabled={submitting}
            required
            autoFocus
          />
        </div>

        <div className="field">
          <label htmlFor="password">Clave</label>
          <input
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={submitting}
            required
          />
        </div>

        <button type="submit" className="btn" disabled={submitting}>
          {submitting ? "Ingresando…" : "Ingresar"}
        </button>
      </form>
    </section>
  );
}
