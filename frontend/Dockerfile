# ---- Compilación ----
FROM node:20-alpine AS build
WORKDIR /app

COPY package.json ./
RUN npm install

COPY . .

# URL del Election Service tal como la ve el navegador (se fija al compilar)
ARG VITE_ELECTION_API_URL=http://localhost:8000
ENV VITE_ELECTION_API_URL=$VITE_ELECTION_API_URL
RUN npm run build

# ---- Servidor estático ----
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
