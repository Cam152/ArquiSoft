// Único lugar que traduce los códigos de respuesta del gateway a mensajes para el
// votante. Lo usan tanto el servidor de Next.js (lib/api/server.ts) como el
// navegador (lib/api/browser.ts), así que no importa nada de Node ni de React.

export type Operation = "elections" | "election" | "results" | "login" | "vote";

/** Error de una llamada al gateway. `status` es 0 si no hubo respuesta. */
export class ApiError extends Error {
  readonly status: number;
  readonly detail: string | null;

  constructor(status: number, message: string, detail: string | null = null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.detail = detail;
  }
}

type Message = string | ((detail: string | null) => string);

const GENERIC: Partial<Record<number, string>> = {
  0: "No se pudo conectar con AGORA. Revisa tu conexión e intenta de nuevo.",
  502: "Un servicio de AGORA no responde. Intenta de nuevo en unos minutos.",
  503: "AGORA no está disponible en este momento. Intenta de nuevo en unos minutos.",
  504: "AGORA tardó demasiado en responder. Intenta de nuevo.",
};

const BY_OPERATION: Record<Operation, Partial<Record<number, Message>>> = {
  elections: {},
  election: { 404: "La elección no existe." },
  results: { 404: "La elección no existe." },
  login: {
    401: "Documento o clave incorrectos.",
    422: "Ingresa tu documento y tu clave.",
  },
  vote: {
    // Sin respuesta: el voto pudo llegar o no. Se le dice al votante cómo saberlo.
    0: "No se pudo confirmar el envío de tu voto. Intenta de nuevo: si aparece «Ya votaste», tu voto sí quedó registrado.",
    401: "Tu sesión expiró. Ingresa de nuevo para votar.",
    404: "La elección no existe.",
    // El gateway responde 409 por dos motivos; el detail dice cuál.
    409: (detail) =>
      detail && /ya vot/i.test(detail)
        ? "Ya votaste en esta elección. Cada votante vota una sola vez por elección."
        : "La elección no está abierta.",
    422: "El candidato no pertenece a esta elección. Recarga la página e intenta de nuevo.",
    500: "No se pudo guardar tu voto y no quedó registrado. Puedes intentarlo de nuevo.",
    // El detail del gateway dice si el voto quedó o no registrado; es más preciso que un texto fijo.
    502: (detail) => detail ?? "El sistema no puede validar tu voto en este momento. Intenta de nuevo en unos minutos.",
  },
};

export function messageFor(operation: Operation, status: number, detail: string | null): string {
  const message = BY_OPERATION[operation][status] ?? GENERIC[status];
  if (typeof message === "function") return message(detail);
  return message ?? detail ?? `Error inesperado (HTTP ${status}).`;
}

/** Red caída o error del servidor: tiene sentido volver a intentar. */
export function isRetryable(status: number): boolean {
  return status === 0 || status >= 500;
}

/** Lee `{"detail": "..."}`, la forma de error común a todos los componentes de AGORA. */
export async function readDetail(response: Response): Promise<string | null> {
  try {
    const body: unknown = await response.json();
    if (body && typeof body === "object" && "detail" in body) {
      const { detail } = body as { detail: unknown };
      if (typeof detail === "string") return detail;
    }
  } catch {
    // Cuerpo vacío o que no es JSON: se usa el mensaje por código.
  }
  return null;
}
