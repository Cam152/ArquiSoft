import type { Metadata } from "next";
import LoginForm from "@/components/LoginForm";
import { safeReturnPath } from "@/lib/navigation";

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
      <LoginForm returnTo={safeReturnPath(volver)} expired={motivo === "expirada"} />
    </section>
  );
}
