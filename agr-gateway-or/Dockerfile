# Node 22 ejecuta TypeScript directamente (quita los tipos al cargar), así que
# no hay paso de compilación ni dependencias que instalar.
FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY package.json ./
COPY src ./src
USER node
EXPOSE 8080
CMD ["node", "--experimental-strip-types", "--disable-warning=ExperimentalWarning", "src/server.ts"]
