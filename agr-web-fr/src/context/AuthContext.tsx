import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { login as loginRequest } from "../services/api";

export interface Session {
  token: string;
  document: string;
  /** Momento (ms desde epoch) en que el JWT deja de ser válido. */
  expiresAt: number;
}

interface AuthContextValue {
  session: Session | null;
  login: (document: string, password: string) => Promise<void>;
  logout: () => void;
}

// sessionStorage: la sesión sobrevive a recargar la página pero se borra al cerrar la pestaña.
const STORAGE_KEY = "votacion.session";
const AuthContext = createContext<AuthContextValue | null>(null);

function readStoredSession(): Session | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const session = JSON.parse(raw) as Partial<Session>;
    if (
      typeof session.token !== "string" ||
      typeof session.document !== "string" ||
      typeof session.expiresAt !== "number" ||
      session.expiresAt <= Date.now()
    ) {
      sessionStorage.removeItem(STORAGE_KEY);
      return null;
    }
    return session as Session;
  } catch {
    return null;
  }
}

function storeSession(session: Session | null) {
  try {
    if (session) sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session));
    else sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Almacenamiento bloqueado (modo privado estricto): la sesión vive solo en memoria.
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(readStoredSession);

  const logout = useCallback(() => {
    storeSession(null);
    setSession(null);
  }, []);

  const login = useCallback(async (document: string, password: string) => {
    const response = await loginRequest(document, password);
    const next: Session = {
      token: response.access_token,
      document,
      expiresAt: Date.now() + (response.expires_in ?? 3600) * 1000,
    };
    storeSession(next);
    setSession(next);
  }, []);

  // Cierra la sesión sola cuando el token expira (1 hora según el contrato).
  useEffect(() => {
    if (!session) return;
    const timer = window.setTimeout(logout, Math.max(session.expiresAt - Date.now(), 0));
    return () => window.clearTimeout(timer);
  }, [session, logout]);

  const value = useMemo(() => ({ session, login, logout }), [session, login, logout]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth debe usarse dentro de <AuthProvider>");
  return context;
}
