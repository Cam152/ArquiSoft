"use client";

// Resultados en vivo. El servidor ya pintó los conteos actuales en el HTML;
// aquí se abre el SSE del gateway y cada evento `counts` reemplaza los votos.
// El evento no trae nombres: se combinan con los candidatos que llegaron con la página.

import { useCallback, useMemo, useState } from "react";
import { tally, type CandidateRef, type Votes } from "@/lib/results";
import { useResultsStream, type StreamStatus } from "@/lib/useResultsStream";
import ResultBars from "./ResultBars";

interface LiveResultsProps {
  electionId: number;
  candidates: CandidateRef[];
  initialVotes: Votes;
  /** URL del SSE, o null si la elección ya no recibe votos (no se abre el stream). */
  streamUrl: string | null;
}

const STATUS_TEXT: Record<StreamStatus, string> = {
  off: "",
  connecting: "Conectando…",
  live: "En vivo",
  reconnecting: "Reconectando…",
};

const timeFormat = new Intl.DateTimeFormat("es-CO", { timeStyle: "medium" });

export default function LiveResults({ electionId, candidates, initialVotes, streamUrl }: LiveResultsProps) {
  const [votes, setVotes] = useState<Votes>(initialVotes);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const onCounts = useCallback((next: Votes) => {
    setVotes(next);
    setUpdatedAt(new Date());
  }, []);

  const status = useResultsStream(streamUrl, electionId, onCounts);
  const { total, rows } = useMemo(() => tally(candidates, votes), [candidates, votes]);

  return (
    <>
      <div className="results-meta">
        <p className="results-total">
          <strong>{total}</strong> {total === 1 ? "voto" : "votos"} en total
        </p>
        {status !== "off" && (
          <span
            className={`live ${status === "live" ? "live-on" : "live-wait"}`}
            role="status"
            aria-live="polite"
          >
            <span className="live-dot" aria-hidden="true" />
            {STATUS_TEXT[status]}
          </span>
        )}
      </div>

      {total === 0 && <p className="muted">Aún no hay votos registrados.</p>}

      <ResultBars rows={rows} />

      {updatedAt && <p className="updated-at">Última actualización: {timeFormat.format(updatedAt)}</p>}
    </>
  );
}
