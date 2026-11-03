"use client";

// Flujo de voto en el navegador: elegir, confirmar y enviar POST /votes al
// gateway con el JWT. La elección y sus candidatos ya llegaron en el HTML que
// generó el servidor; aquí solo vive la interacción.

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { castVote } from "@/lib/api/browser";
import { ApiError, isRetryable } from "@/lib/api/errors";
import type { Candidate } from "@/lib/api/types";
import { loginPath } from "@/lib/navigation";
import { clearSession, useSession } from "@/lib/session";
import Notice from "./Notice";

interface VoteFormProps {
  gatewayUrl: string;
  electionId: number;
  candidates: Candidate[];
}

type Step = "choose" | "confirm" | "sending" | "done";

interface VoteError {
  message: string;
  retryable: boolean;
}

export default function VoteForm({ gatewayUrl, electionId, candidates }: VoteFormProps) {
  const router = useRouter();
  const { ready, session } = useSession();
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [step, setStep] = useState<Step>("choose");
  const [error, setError] = useState<VoteError | null>(null);

  const votePath = `/elecciones/${electionId}/votar`;
  const resultsPath = `/elecciones/${electionId}/resultados`;
  const selected = candidates.find((c) => c.id === selectedId) ?? null;

  // Si al abrir la página no hay sesión, se va al login y luego se vuelve aquí.
  // Si la sesión se cierra después (botón "Salir" o 401), quien la cerró decide a
  // dónde ir; por eso solo se redirige cuando nunca hubo sesión en esta página.
  const hadSession = useRef(false);
  useEffect(() => {
    if (session) hadSession.current = true;
    else if (ready && !hadSession.current) router.replace(loginPath(votePath));
  }, [ready, session, router, votePath]);

  async function submit() {
    if (!session || !selected) return;
    setStep("sending");
    setError(null);
    try {
      await castVote(gatewayUrl, session.token, electionId, selected.id);
      setStep("done");
    } catch (err) {
      const apiError = err instanceof ApiError ? err : new ApiError(0, "Ocurrió un error inesperado.");
      if (apiError.status === 401) {
        // JWT vencido o inválido: se cierra la sesión y se pide ingresar de nuevo.
        router.replace(loginPath(votePath, true));
        clearSession();
        return;
      }
      setError({ message: apiError.message, retryable: isRetryable(apiError.status) });
      setStep("confirm");
    }
  }

  if (!ready) return <p className="muted">Verificando tu sesión…</p>;

  if (!session && step !== "done") {
    return (
      <Notice tone="info" title="Necesitas ingresar para votar">
        <div className="actions">
          <Link href={loginPath(votePath)} className="btn">
            Ingresar
          </Link>
        </div>
      </Notice>
    );
  }

  if (step === "done") {
    return (
      <Notice tone="success" title="Voto registrado">
        <p>Tu voto quedó guardado de forma anónima. Gracias por participar.</p>
        <div className="actions">
          <Link href={resultsPath} className="btn">
            Ver resultados en vivo
          </Link>
          <Link href="/" className="btn btn-secondary">
            Volver a elecciones
          </Link>
        </div>
      </Notice>
    );
  }

  // Rechazos definitivos (409, 404, 422): no se vuelve a mostrar el formulario.
  if (error && !error.retryable) {
    return (
      <Notice tone="warn" title="No se registró el voto">
        <p>{error.message}</p>
        <div className="actions">
          <Link href={resultsPath} className="btn">
            Ver resultados
          </Link>
          <Link href="/" className="btn btn-secondary">
            Volver a elecciones
          </Link>
        </div>
      </Notice>
    );
  }

  if ((step === "confirm" || step === "sending") && selected) {
    const sending = step === "sending";
    return (
      <div className="card confirm">
        <p className="muted">Vas a votar por</p>
        <p className="confirm-choice">
          <span className="candidate-number">{selected.number}</span>
          {selected.name}
        </p>
        <p className="muted">Una vez enviado, el voto no se puede cambiar.</p>

        {error && (
          <Notice tone="error">
            <p>{error.message}</p>
          </Notice>
        )}

        <div className="actions">
          <button type="button" className="btn" onClick={submit} disabled={sending}>
            {sending ? "Enviando voto…" : error ? "Intentar de nuevo" : "Confirmar voto"}
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={sending}
            onClick={() => {
              setError(null);
              setStep("choose");
            }}
          >
            Cambiar
          </button>
        </div>
      </div>
    );
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (selected) setStep("confirm");
      }}
    >
      <fieldset className="options-fieldset">
        <legend>Elige un candidato</legend>
        <ul className="options">
          {candidates.map((candidate) => (
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
                  {candidate.description && <span className="muted candidate-note">{candidate.description}</span>}
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
  );
}
