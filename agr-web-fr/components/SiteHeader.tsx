import Link from "next/link";
import SessionMenu from "./SessionMenu";

/** Encabezado común. Se genera en el servidor; solo SessionMenu corre en el navegador. */
export default function SiteHeader() {
  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link href="/" className="brand">
          <span className="brand-mark" aria-hidden="true" />
          AGORA
          <span className="brand-tag">Votación electrónica</span>
        </Link>
        <div className="session">
          <SessionMenu />
        </div>
      </div>
    </header>
  );
}
