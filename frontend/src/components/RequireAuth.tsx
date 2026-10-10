import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

/** Estado que recibe /ingresar: a dónde volver y si se llegó por sesión expirada. */
export interface LoginRedirectState {
  from?: string;
  expired?: boolean;
}

/** Protege una ruta: sin sesión, manda al login y vuelve aquí después de ingresar. */
export default function RequireAuth({ children }: { children: ReactNode }) {
  const { session } = useAuth();
  const location = useLocation();

  if (!session) {
    const state: LoginRedirectState = { from: location.pathname };
    return <Navigate to="/ingresar" replace state={state} />;
  }
  return <>{children}</>;
}
