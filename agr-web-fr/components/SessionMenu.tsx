"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { loginPath } from "@/lib/navigation";
import { clearSession, useSession } from "@/lib/session";

/**
 * Parte del encabezado que depende de la sesión. La sesión vive en el navegador,
 * así que el servidor deja un espacio reservado y esto se completa al hidratar.
 */
export default function SessionMenu() {
  const { ready, session } = useSession();
  const pathname = usePathname();
  const router = useRouter();

  if (!ready) return <span className="session-placeholder" aria-hidden="true" />;

  if (!session) {
    // En /ingresar el formulario ya está en pantalla.
    if (pathname === "/ingresar") return <span className="session-placeholder" aria-hidden="true" />;
    return (
      <Link href={loginPath(pathname)} className="btn btn-secondary btn-small">
        Ingresar
      </Link>
    );
  }

  return (
    <>
      <span className="session-user">Documento {session.document}</span>
      <button
        type="button"
        className="btn btn-ghost"
        onClick={() => {
          clearSession();
          router.push("/");
        }}
      >
        Salir
      </button>
    </>
  );
}
