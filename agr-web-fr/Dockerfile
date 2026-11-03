# agr-web-fr: Next.js con SSR. Tres etapas: la imagen final no lleva el
# compilador ni node_modules completo, solo la salida "standalone" de Next.js.
# Node 22 (LTS), igual que agr-gateway-or. Node 20 dejó de recibir soporte en abril de 2026.

# ---- Dependencias ----
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

# ---- Compilación ----
FROM node:22-alpine AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# No hace falta el gateway para compilar: las páginas se generan en cada petición.
RUN npm run build

# ---- Ejecución ----
FROM node:22-alpine AS run
WORKDIR /app
# HOSTNAME=0.0.0.0: sin esto, server.js escucha solo en la IP del contenedor y el
# healthcheck contra 127.0.0.1 falla.
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
# Si más adelante agregan archivos en public/ (logo, favicon), copien también:
# COPY --from=build --chown=node:node /app/public ./public
USER node
EXPOSE 3000
# GATEWAY_INTERNAL_URL y NEXT_PUBLIC_GATEWAY_URL se leen al ejecutar (docker-compose.yml).
CMD ["node", "server.js"]
