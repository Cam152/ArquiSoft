import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import SiteHeader from "@/components/SiteHeader";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "AGORA", template: "%s · AGORA" },
  description: "AGORA: votación electrónica para elecciones estudiantiles.",
};

export const viewport: Viewport = {
  themeColor: "#2f6b3f",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="es">
      <body>
        <a href="#contenido" className="skip-link">
          Saltar al contenido
        </a>
        <SiteHeader />
        <main id="contenido" className="page">
          {children}
        </main>
        <footer className="site-footer">AGORA · Arquitectura de Software 2026-II · Equipo 1b</footer>
      </body>
    </html>
  );
}
