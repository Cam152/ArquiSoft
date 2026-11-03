// Utilidades de navegación sin dependencias (se pueden probar con node --test).

/**
 * Ruta a la que se vuelve después del login. Solo acepta rutas internas
 * ("/elecciones/1/votar"); cualquier otra cosa ("//sitio.com", "https://...")
 * se cambia por "/" para que el enlace de login no sirva para redirigir afuera.
 */
export function safeReturnPath(raw: string | string[] | undefined | null): string {
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (!value || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return "/";
  return value;
}

/** Id de elección de la URL. Devuelve null si no es un entero positivo de 32 bits (Int de GraphQL). */
export function parseElectionId(raw: string): number | null {
  if (!/^\d{1,10}$/.test(raw)) return null;
  const id = Number(raw);
  return id > 0 && id <= 2_147_483_647 ? id : null;
}

export function loginPath(returnTo: string, expired = false): string {
  const params = new URLSearchParams({ volver: returnTo });
  if (expired) params.set("motivo", "expirada");
  return `/ingresar?${params.toString()}`;
}
