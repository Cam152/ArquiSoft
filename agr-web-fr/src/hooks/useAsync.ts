import { useCallback, useEffect, useState, type DependencyList } from "react";
import { toApiError, type ApiError } from "../services/api";

export type AsyncState<T> =
  | { status: "loading" }
  | { status: "success"; data: T }
  | { status: "error"; error: ApiError };

/**
 * Ejecuta una petición al montar (y cuando cambian `deps`) y expone su estado.
 * Devuelve `[state, reload]`. Ignora respuestas que llegan después de desmontar.
 */
export function useAsync<T>(
  load: () => Promise<T>,
  deps: DependencyList,
): [AsyncState<T>, () => void] {
  const [state, setState] = useState<AsyncState<T>>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
    load()
      .then((data) => {
        if (!cancelled) setState({ status: "success", data });
      })
      .catch((error: unknown) => {
        if (!cancelled) setState({ status: "error", error: toApiError(error) });
      });
    return () => {
      cancelled = true;
    };
    // `load` cambia en cada render; lo que decide cuándo recargar son `deps` y `attempt`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);
  return [state, reload];
}
