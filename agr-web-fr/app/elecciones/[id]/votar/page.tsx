import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import Notice from "@/components/Notice";
import PageError from "@/components/PageError";
import VoteForm from "@/components/VoteForm";
import { getElection } from "@/lib/api/server";
import type { Election } from "@/lib/api/types";
import { gatewayPublicUrl } from "@/lib/config";
import { parseElectionId } from "@/lib/navigation";

export const dynamic = "force-dynamic";

export const metadata: Metadata = { title: "Votar" };

interface VotePageProps {
  params: Promise<{ id: string }>;
}

// Servidor: trae la elección y sus candidatos (GraphQL al gateway) y arma el HTML.
// Navegador: VoteForm envía POST /votes al gateway con el JWT.
export default async function VotePage({ params }: VotePageProps) {
  const { id } = await params;
  const electionId = parseElectionId(id);
  if (electionId === null) notFound();

  let election: Election | null;
  try {
    election = await getElection(electionId);
  } catch (error) {
    return <PageError title="No se pudo cargar la elección" error={error} backLink />;
  }
  if (!election) notFound();

  const resultsPath = `/elecciones/${election.id}/resultados`;

  return (
    <>
      <Link href="/" className="back-link">
        ← Elecciones
      </Link>
      <header className="page-header">
        <h1>Votar</h1>
        <p>{election.name}</p>
      </header>

      {election.isOpen ? (
        <VoteForm gatewayUrl={gatewayPublicUrl()} electionId={election.id} candidates={election.candidates} />
      ) : (
        <Notice tone="info" title="Esta elección no está abierta">
          <p>Solo se puede votar mientras la elección esté abierta.</p>
          {election.status !== "draft" && (
            <div className="actions">
              <Link href={resultsPath} className="btn btn-secondary">
                Ver resultados
              </Link>
            </div>
          )}
        </Notice>
      )}
    </>
  );
}
