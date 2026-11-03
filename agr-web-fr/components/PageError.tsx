import Link from "next/link";
import { ApiError } from "@/lib/api/errors";
import Notice from "./Notice";
import RetryButton from "./RetryButton";

interface PageErrorProps {
  title: string;
  error: unknown;
  /** Muestra el enlace de vuelta a la lista de elecciones. */
  backLink?: boolean;
}

/**
 * Error de una página generada en el servidor. Muestra el mensaje de lib/api/errors.ts
 * (pensado para el votante); cualquier otro error se reemplaza por un texto genérico
 * para no exponer detalles internos.
 */
export default function PageError({ title, error, backLink = false }: PageErrorProps) {
  const message =
    error instanceof ApiError ? error.message : "Ocurrió un error inesperado al generar la página.";
  if (!(error instanceof ApiError)) console.error(error);

  return (
    <>
      {backLink && (
        <Link href="/" className="back-link">
          ← Elecciones
        </Link>
      )}
      <Notice tone="error" title={title}>
        <p>{message}</p>
        <div className="actions">
          <RetryButton />
        </div>
      </Notice>
    </>
  );
}
