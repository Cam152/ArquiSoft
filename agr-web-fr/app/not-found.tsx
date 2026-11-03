import Link from "next/link";
import Notice from "@/components/Notice";

export default function NotFound() {
  return (
    <Notice tone="warn" title="No encontramos lo que buscas">
      <p>La elección o la página no existe.</p>
      <div className="actions">
        <Link href="/" className="btn">
          Ver elecciones
        </Link>
      </div>
    </Notice>
  );
}
