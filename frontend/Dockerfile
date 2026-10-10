# ---- Compilación ----
FROM node:20-alpine AS build
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

COPY . .

# URLs de los servicios tal como las ve el navegador (se fijan al compilar)
ARG VITE_ELECTION_API_URL=http://localhost:8000
ARG VITE_VOTER_API_URL=http://localhost:8001
ARG VITE_VOTE_API_URL=http://localhost:8002
ENV VITE_ELECTION_API_URL=$VITE_ELECTION_API_URL
ENV VITE_VOTER_API_URL=$VITE_VOTER_API_URL
ENV VITE_VOTE_API_URL=$VITE_VOTE_API_URL
RUN npm run build

# ---- Servidor estático ----
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
