"use client";

// Último recurso para errores no previstos al generar una página. Los errores del
// gateway ya se muestran dentro de cada página con PageError.

import Link from "next/link";
import Notice from "@/components/Notice";

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <Notice tone="error" title="Algo salió mal">
      <p>No pudimos mostrar esta página. Intenta de nuevo en unos segundos.</p>
      <div className="actions">
        <button type="button" className="btn" onClick={reset}>
          Reintentar
        </button>
        <Link href="/" className="btn btn-secondary">
          Ver elecciones
        </Link>
      </div>
    </Notice>
  );
}
