import type { ElectionStatus } from "@/lib/api/types";

interface StatusBadgeProps {
  status: ElectionStatus;
  /** Una elección activa fuera de su horario (starts_at, ends_at) no recibe votos. */
  isOpen: boolean;
}

export default function StatusBadge({ status, isOpen }: StatusBadgeProps) {
  if (status === "active" && !isOpen) {
    return <span className="status status-paused">Fuera de horario</span>;
  }
  const label = { draft: "En preparación", active: "Abierta", closed: "Cerrada" }[status];
  return <span className={`status status-${status}`}>{label}</span>;
}
