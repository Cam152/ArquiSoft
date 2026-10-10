import type { ReactNode } from "react";

type Tone = "info" | "success" | "warn" | "error";

interface NoticeProps {
  tone?: Tone;
  title?: string;
  children?: ReactNode;
}

export default function Notice({ tone = "info", title, children }: NoticeProps) {
  return (
    <div className={`notice notice-${tone}`} role={tone === "error" ? "alert" : "status"}>
      {title && <p className="notice-title">{title}</p>}
      {children}
    </div>
  );
}
