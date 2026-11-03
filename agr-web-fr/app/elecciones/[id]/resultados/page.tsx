import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import LiveResults from "@/components/LiveResults";
import Notice from "@/components/Notice";
import PageError from "@/components/PageError";
import StatusBadge from "@/components/StatusBadge";
import { resultsStreamUrl } from "@/lib/api/browser";
import { ApiError } from "@/lib/api/errors";
import { getElection, getResults } from "@/lib/api/server";
import type { Election, ElectionResults } from "@/lib/api/types";
import { gatewayPublicUrl } from "@/lib/config";
import { parseElectionId } from "@/lib/navigation";
import { votesFromResults } from "@/lib/results";

export const dynamic = "force-dynamic";

export const metadata: Metadata = { title: "Resultados" };

interface ResultsPageProps {
  params: Promise<{ id: string }>;
}

// Servidor: trae en paralelo los resultados actuales (composición del gateway) y la
// elección con sus candidatos, y pinta las barras en el HTML.
// Navegador: LiveResults abre GET /results/{id}/stream (SSE) y actualiza las barras.
export default async function ResultsPage({ params }: ResultsPageProps) {
  const { id } = await params;
  const electionId = parseElectionId(id);
  if (electionId === null) notFound();

  let results: ElectionResults;
  let election: Election | null;
  try {
    [results, election] = await Promise.all([getResults(electionId), getElection(electionId)]);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    return <PageError title="No se pudieron cargar los resultados" error={error} backLink />;
  }
  if (!election) notFound();

  const candidates = election.candidates.map(({ id: candidateId, number, name }) => ({ id: candidateId, number, name }));
  // Solo una elección activa recibe votos; para las cerradas no se abre el stream.
  const streamUrl = election.status === "active" ? resultsStreamUrl(gatewayPublicUrl(), election.id) : null;

  return (
    <>
      <Link href="/" className="back-link">
        ← Elecciones
      </Link>
      <header className="page-header">
        <div className="election-head">
          <h1>Resultados</h1>
          <StatusBadge status={election.status} isOpen={election.isOpen} />
        </div>
        <p>{results.election_name || election.name}</p>
      </header>

      {election.status === "draft" ? (
        <Notice tone="info" title="Esta elección todavía no ha empezado">
          <p>Los resultados aparecerán cuando se abra la votación.</p>
        </Notice>
      ) : (
        <>
          {election.status === "active" ? (
            <Notice tone="info">
              <p>La votación sigue abierta: los resultados se actualizan solos con cada voto.</p>
            </Notice>
          ) : (
            <Notice tone="success">
              <p>La votación terminó. Estos son los resultados finales.</p>
            </Notice>
          )}
          <LiveResults
            electionId={election.id}
            candidates={candidates}
            initialVotes={votesFromResults(results.results)}
            streamUrl={streamUrl}
          />
        </>
      )}
    </>
  );
}
