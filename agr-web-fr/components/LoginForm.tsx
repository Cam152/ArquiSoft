"use client";

// Login desde el navegador (Prototipo 3, S2 y S4). El formulario llama a
// POST {gatewayUrl}/auth/login, que con gatewayUrl = "/api" es /api/auth/login en
// agr-proxy-rp (mismo origen). Así el límite de intentos del proxy se aplica a la
// IP real de cada votante y no a la del contenedor de la web.

import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { login } from "@/lib/api/browser";
import { ApiError } from "@/lib/api/errors";
import { saveSession, useSession } from "@/lib/session";
import Notice from "./Notice";

interface LoginFormProps {
  /** Gateway visto desde el navegador (NEXT_PUBLIC_GATEWAY_URL, por defecto "/api"). */
  gatewayUrl: string;
  /** Ruta interna a la que se vuelve al ingresar (ya validada en el servidor). */
  returnTo: string;
  /** Se llegó aquí porque el JWT venció mientras votaba. */
  expired: boolean;
}

type FormStatus =
  | { kind: "idle" }
  | { kind: "sending" }
  | { kind: "error"; message: string }
  | { kind: "success" };

const MAX_LENGTH = 64;

export default function LoginForm({ gatewayUrl, returnTo, expired }: LoginFormProps) {
  const router = useRouter();
  const { ready, session } = useSession();
  const [status, setStatus] = useState<FormStatus>({ kind: "idle" });
  // Controlados para conservar el documento y borrar solo la clave cuando falla.
  const [documentNumber, setDocumentNumber] = useState("");
  const [password, setPassword] = useState("");

  // Con sesión activa (también justo después de ingresar) no tiene sentido el formulario.
  useEffect(() => {
    if (ready && session) router.replace(returnTo);
  }, [ready, session, returnTo, router]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (status.kind === "sending") return;

    const document = documentNumber.trim();
    if (!document || !password) {
      setStatus({ kind: "error", message: "Ingresa tu documento y tu clave." });
      return;
    }
    if (document.length > MAX_LENGTH || password.length > MAX_LENGTH) {
      setStatus({ kind: "error", message: "Documento o clave incorrectos." });
      return;
    }

    setStatus({ kind: "sending" });
    try {
      const response = await login(gatewayUrl, document, password);
      // El token queda en sessionStorage; el efecto de arriba hace la redirección.
      saveSession(response.access_token, document, response.expires_in);
      setStatus({ kind: "success" });
    } catch (error) {
      setPassword("");
      if (!(error instanceof ApiError)) console.error(error);
      setStatus({
        kind: "error",
        message: error instanceof ApiError ? error.message : "No se pudo iniciar sesión. Intenta de nuevo.",
      });
    }
  }

  const redirecting = status.kind === "success" || (ready && session !== null);
  const busy = status.kind === "sending" || redirecting;
  // Mientras React no hidrata (ready = false) el botón queda deshabilitado, y con él
  // el envío con Enter: el formulario nunca sale como HTML plano sin JavaScript.
  // method="post" es un respaldo para que la clave jamás termine en la URL.
  const canSubmit = ready && !busy;

  return (
    <>
      {expired && status.kind !== "error" && (
        <Notice tone="warn">
          <p>Tu sesión expiró. Ingresa de nuevo para continuar.</p>
        </Notice>
      )}

      {status.kind === "error" && (
        <Notice tone="error">
          <p>{status.message}</p>
        </Notice>
      )}

      <form className="card form" method="post" onSubmit={handleSubmit}>
        <div className="field">
          <label htmlFor="document">Número de documento</label>
          <input
            id="document"
            name="document"
            inputMode="numeric"
            autoComplete="username"
            value={documentNumber}
            onChange={(e) => setDocumentNumber(e.target.value)}
            disabled={busy}
            required
            maxLength={MAX_LENGTH}
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
            disabled={busy}
            required
            maxLength={MAX_LENGTH}
          />
        </div>

        <button type="submit" className="btn" disabled={!canSubmit}>
          {redirecting ? "Entrando…" : status.kind === "sending" ? "Ingresando…" : "Ingresar"}
        </button>
      </form>
    </>
  );
}
