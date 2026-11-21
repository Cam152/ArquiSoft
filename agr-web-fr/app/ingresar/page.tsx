import type { Metadata } from "next";
import LoginForm from "@/components/LoginForm";
import { gatewayPublicUrl } from "@/lib/config";
import { safeReturnPath } from "@/lib/navigation";

// La URL pública del gateway se lee al ejecutar, como en /votar y /resultados.
export const dynamic = "force-dynamic";

export const metadata: Metadata = { title: "Ingresar" };

interface LoginPageProps {
  searchParams: Promise<{ volver?: string | string[]; motivo?: string | string[] }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const { volver, motivo } = await searchParams;

  return (
    <section className="narrow">
      <header className="page-header">
        <h1>Ingresar</h1>
        <p>Usa tu número de documento y la clave que te asignaron.</p>
      </header>
      <LoginForm
        gatewayUrl={gatewayPublicUrl()}
        returnTo={safeReturnPath(volver)}
        expired={motivo === "expirada"}
      />
    </section>
  );
}
