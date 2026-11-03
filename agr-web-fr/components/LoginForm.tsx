"use client";

import { useRouter } from "next/navigation";
import { useActionState, useEffect, useState } from "react";
import { loginAction, type LoginState } from "@/app/ingresar/actions";
import { saveSession, useSession } from "@/lib/session";
import Notice from "./Notice";

interface LoginFormProps {
  /** Ruta interna a la que se vuelve al ingresar (ya validada en el servidor). */
  returnTo: string;
  /** Se llegó aquí porque el JWT venció mientras votaba. */
  expired: boolean;
}

const INITIAL: LoginState = { status: "idle" };

export default function LoginForm({ returnTo, expired }: LoginFormProps) {
  const router = useRouter();
  const { ready, session } = useSession();
  const [state, formAction, pending] = useActionState(loginAction, INITIAL);
  // Controlado para que el documento no se borre si la clave es incorrecta.
  const [documentNumber, setDocumentNumber] = useState("");

  // Login correcto: el token queda en sessionStorage y se vuelve a donde se venía.
  useEffect(() => {
    if (state.status === "success") {
      saveSession(state.token, state.document, state.expiresIn);
    }
  }, [state]);

  // Con sesión activa (también justo después de ingresar) no tiene sentido el formulario.
  useEffect(() => {
    if (ready && session) router.replace(returnTo);
  }, [ready, session, returnTo, router]);

  const redirecting = state.status === "success" || (ready && session !== null);
  const busy = pending || redirecting;

  return (
    <>
      {expired && state.status !== "error" && (
        <Notice tone="warn">
          <p>Tu sesión expiró. Ingresa de nuevo para continuar.</p>
        </Notice>
      )}

      {state.status === "error" && (
        <Notice tone="error">
          <p>{state.message}</p>
        </Notice>
      )}

      <form className="card form" action={formAction}>
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
            maxLength={64}
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
            disabled={busy}
            required
            maxLength={64}
          />
        </div>

        <button type="submit" className="btn" disabled={busy}>
          {redirecting ? "Entrando…" : pending ? "Ingresando…" : "Ingresar"}
        </button>
      </form>
    </>
  );
}
