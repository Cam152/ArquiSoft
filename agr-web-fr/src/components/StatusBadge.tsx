import type { ElectionStatus } from "../services/api";

const STATUS_LABEL: Record<ElectionStatus, string> = {
  draft: "En preparación",
  active: "Abierta",
  closed: "Cerrada",
};

export default function StatusBadge({ status }: { status: ElectionStatus }) {
  return <span className={`status status-${status}`}>{STATUS_LABEL[status]}</span>;
}
