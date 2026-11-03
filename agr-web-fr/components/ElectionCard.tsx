import Link from "next/link";
import type { Election } from "@/lib/api/types";
import StatusBadge from "./StatusBadge";

// El servidor del contenedor corre en UTC; las fechas se muestran en hora de Colombia.
const dateFormat = new Intl.DateTimeFormat("es-CO", {
  dateStyle: "long",
  timeStyle: "short",
  timeZone: "America/Bogota",
});

function schedule(election: Election): string | null {
  const starts = election.startsAt ? dateFormat.format(new Date(election.startsAt)) : null;
  const ends = election.endsAt ? dateFormat.format(new Date(election.endsAt)) : null;
  if (starts && ends) return `Del ${starts} al ${ends}`;
  if (ends) return `Hasta el ${ends}`;
  if (starts) return `Desde el ${starts}`;
  return null;
}

/** Tarjeta de una elección con sus candidatos. Componente de servidor: no envía JavaScript. */
export default function ElectionCard({ election }: { election: Election }) {
  const when = schedule(election);

  return (
    <li className="election">
      <div className="election-head">
        <h2>{election.name}</h2>
        <StatusBadge status={election.status} isOpen={election.isOpen} />
      </div>

      {election.description && <p className="muted">{election.description}</p>}
      {when && <p className="dates">{when}</p>}

      <ol className="candidates" aria-label={`Candidatos de ${election.name}`}>
        {election.candidates.map((candidate) => (
          <li key={candidate.id}>
            <span className="candidate-number">{candidate.number}</span>
            <span>
              {candidate.name}
              {candidate.description && <span className="muted candidate-note">{candidate.description}</span>}
            </span>
          </li>
        ))}
      </ol>

      <div className="actions">
        {election.isOpen && (
          <Link href={`/elecciones/${election.id}/votar`} className="btn">
            Votar
          </Link>
        )}
        {election.status !== "draft" && (
          <Link href={`/elecciones/${election.id}/resultados`} className="btn btn-secondary">
            {election.status === "active" ? "Resultados en vivo" : "Ver resultados"}
          </Link>
        )}
      </div>
    </li>
  );
}
