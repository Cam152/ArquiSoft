// Healthcheck del contenedor (docker-compose.yml). Solo dice si el servidor de
// Next.js responde; no consulta al gateway, para que una falla aguas abajo no
// marque al web como caído.

export const dynamic = "force-dynamic";

export function GET() {
  return Response.json({ status: "ok" });
}
