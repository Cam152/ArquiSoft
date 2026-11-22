#!/usr/bin/env bash
# p3/s1/gen-certs.sh · S1 Secure Channel (responsable: Miguel)
#
# Genera el certificado autofirmado de DESARROLLO que usa agr-proxy-rp para TLS:
#   p3/s1/certs/localhost.key   llave privada (NO se sube al repositorio: .gitignore)
#   p3/s1/certs/localhost.crt   certificado público (tampoco se versiona; se genera en cada equipo)
#
# Subject Alternative Names: localhost, 127.0.0.1 y ::1.
# docker-compose.yml monta p3/s1/certs/ en /etc/nginx/certs como SOLO LECTURA; los
# archivos no se copian a la imagen del proxy.
#
# Uso (desde la raíz del repo, en Git Bash, WSL o Linux/macOS):
#   bash p3/s1/gen-certs.sh            crea el certificado si no existe
#   bash p3/s1/gen-certs.sh --force    lo vuelve a crear (por ejemplo, si venció)
#
# Variables opcionales:  CERT_DIR (carpeta de salida)  CERT_DAYS (vigencia, por defecto 365)
#
# Este certificado NO es de confianza pública: el navegador mostrará un aviso la primera
# vez y hay que aceptarlo (o importar localhost.crt como autoridad de confianza). En
# producción se usaría un certificado emitido por una CA (por ejemplo, Let's Encrypt).
set -euo pipefail

DIR=$(cd "$(dirname "$0")" && pwd)
OUT="${CERT_DIR:-$DIR/certs}"
DAYS="${CERT_DAYS:-365}"
FORCE=0
[[ "${1:-}" == "--force" ]] && FORCE=1

if ! command -v openssl >/dev/null 2>&1; then
  echo "ERROR: no se encontró openssl. En Windows use Git Bash (viene con Git for Windows)." >&2
  exit 1
fi

show() {
  echo
  echo "Certificado: $OUT/localhost.crt"
  openssl x509 -in "$OUT/localhost.crt" -noout -subject -issuer -dates -fingerprint -sha256 -ext subjectAltName
}

if [[ -f "$OUT/localhost.crt" && -f "$OUT/localhost.key" && $FORCE -eq 0 ]]; then
  echo "Ya existen la llave y el certificado en $OUT (use --force para regenerarlos)."
  show
  exit 0
fi

mkdir -p "$OUT"
CONF=$(mktemp)
trap 'rm -f "$CONF"' EXIT

cat >"$CONF" <<'EOF'
[req]
distinguished_name = dn
x509_extensions    = v3
prompt             = no

[dn]
C  = CO
O  = AGORA (desarrollo)
CN = localhost

[v3]
subjectAltName   = DNS:localhost,IP:127.0.0.1,IP:::1
basicConstraints = critical,CA:FALSE
keyUsage         = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
EOF

# Se genera en nombres temporales y se mueve al final: nunca queda un par a medias.
# (Los errores de OpenSSL se muestran: si falla, el mensaje dice por que.)
echo "Generando llave y certificado en $OUT ..."
openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days "$DAYS" \
  -config "$CONF" -keyout "$OUT/localhost.key.tmp" -out "$OUT/localhost.crt.tmp"

mv -f "$OUT/localhost.key.tmp" "$OUT/localhost.key"
mv -f "$OUT/localhost.crt.tmp" "$OUT/localhost.crt"
chmod 600 "$OUT/localhost.key" 2>/dev/null || true   # en NTFS no tiene efecto
chmod 644 "$OUT/localhost.crt" 2>/dev/null || true

# La llave y el certificado deben ser un par.
key_pub=$(openssl pkey -in "$OUT/localhost.key" -pubout 2>/dev/null | openssl dgst -sha256)
crt_pub=$(openssl x509 -in "$OUT/localhost.crt" -noout -pubkey 2>/dev/null | openssl dgst -sha256)
if [[ "$key_pub" != "$crt_pub" ]]; then
  echo "ERROR: la llave y el certificado generados no coinciden." >&2
  exit 1
fi

echo "Certificado de desarrollo generado en $OUT"
show
echo
echo "La llave privada (localhost.key) está en .gitignore: no la suba al repositorio."
