"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";

/** Vuelve a generar la página en el servidor (nueva consulta al gateway) sin recargar todo. */
export default function RetryButton({ label = "Reintentar" }: { label?: string }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  return (
    <button
      type="button"
      className="btn"
      disabled={pending}
      onClick={() => startTransition(() => router.refresh())}
    >
      {pending ? "Reintentando…" : label}
    </button>
  );
}
