"use client";

// Sesión del votante en el navegador. El JWT se guarda en sessionStorage: sobrevive
// a recargar la página y se borra al cerrar la pestaña. Las páginas que se generan
// en el servidor no lo necesitan (elecciones y resultados son públicos); solo lo
// usa el navegador para POST /votes.

import { useSyncExternalStore } from "react";

export interface Session {
  token: string;
  document: string;
  /** Momento (ms desde epoch) en que el JWT deja de servir. */
  expiresAt: number;
}

const STORAGE_KEY = "agora.session";

// undefined = todavía no se leyó sessionStorage.
let current: Session | null | undefined;
let expiryTimer: ReturnType<typeof setTimeout> | undefined;
const listeners = new Set<() => void>();

function readStored(): Session | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const value = JSON.parse(raw) as Partial<Session>;
    if (
      typeof value.token === "string" &&
      typeof value.document === "string" &&
      typeof value.expiresAt === "number" &&
      value.expiresAt > Date.now()
    ) {
      return value as Session;
    }
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Almacenamiento bloqueado o dato corrupto: no hay sesión.
  }
  return null;
}

function store(session: Session | null) {
  try {
    if (session) sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session));
    else sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Si el navegador bloquea el almacenamiento, la sesión vive solo en memoria.
  }
}

/** Programa el cierre automático de la sesión cuando vence el JWT. */
function scheduleExpiry() {
  clearTimeout(expiryTimer);
  if (current) {
    expiryTimer = setTimeout(clearSession, Math.max(current.expiresAt - Date.now(), 0));
  }
}

function setCurrent(session: Session | null) {
  current = session;
  store(session);
  scheduleExpiry();
  listeners.forEach((listener) => listener());
}

function getSnapshot(): Session | null {
  if (current === undefined) {
    current = readStored();
    scheduleExpiry();
  }
  return current;
}

// En el servidor (y durante la hidratación) la sesión es desconocida.
function getServerSnapshot(): undefined {
  return undefined;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function saveSession(token: string, document: string, expiresInSeconds: number) {
  const seconds = Number.isFinite(expiresInSeconds) && expiresInSeconds > 0 ? expiresInSeconds : 3600;
  setCurrent({ token, document, expiresAt: Date.now() + seconds * 1000 });
}

export function clearSession() {
  setCurrent(null);
}

/**
 * Sesión actual. `ready` es false mientras React hidrata la página generada en el
 * servidor; en ese momento todavía no se sabe si hay sesión.
 */
export function useSession(): { ready: boolean; session: Session | null } {
  const value = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  return { ready: value !== undefined, session: value ?? null };
}
