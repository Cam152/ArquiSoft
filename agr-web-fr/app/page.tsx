import type { Metadata } from "next";
import ElectionCard from "@/components/ElectionCard";
import PageError from "@/components/PageError";
import { getElections } from "@/lib/api/server";
import type { Election } from "@/lib/api/types";

// SSR en cada petición: el HTML sale con los datos del momento y `next build` no
// necesita al gateway.
export const dynamic = "force-dynamic";

export const metadata: Metadata = { title: "Elecciones" };

// Primero las abiertas, luego las que vienen y al final las cerradas.
const ORDER = { active: 0, draft: 1, closed: 2 } as const;

export default async function ElectionsPage() {
  let elections: Election[];
  try {
    elections = await getElections();
  } catch (error) {
    return <PageError title="No se pudieron cargar las elecciones" error={error} />;
  }

  const sorted = [...elections].sort((a, b) => ORDER[a.status] - ORDER[b.status] || a.id - b.id);

  return (
    <>
      <header className="page-header">
        <h1>Elecciones</h1>
        <p>Elecciones registradas y sus candidatos.</p>
      </header>

      {sorted.length === 0 ? (
        <p className="muted">Todavía no hay elecciones registradas.</p>
      ) : (
        <ul className="elections">
          {sorted.map((election) => (
            <ElectionCard key={election.id} election={election} />
          ))}
        </ul>
      )}
    </>
  );
}
