# Sistema de votación — Prototipo 1

Proyecto de Arquitectura de Software (SwArch 2026-II, UNAL).

## Planeación del prototipo

El prototipo es **vertical**: una sola funcionalidad que atraviese todas las capas, en vez de muchas funciones a medias. Para un sistema de votación, se acota al flujo **"un votante emite su voto y se ven los resultados"**.

## Alcance funcional

**Dentro del prototipo:**

- Ingreso simple del votante (login básico o identificación por documento).
- Ver una elección activa con sus candidatos.
- Emitir un voto, con una regla de negocio: un voto por votante.
- Consultar los resultados.

**Fuera del prototipo (trabajo futuro):** panel de administración completo, múltiples elecciones simultáneas, biometría, blockchain, alta disponibilidad, seguridad avanzada.

## Mapeo del alcance a los requisitos

| Requisito | Propuesta |
|---|---|
| Presentación | Front-end web (React o similar) con pantallas de login, votación y resultados |
| Lógica 1 | Servicio de elecciones y votantes: autenticación, elecciones, candidatos, control de "ya votó" |
| Lógica 2 | Servicio de votos y resultados: registra votos y calcula el conteo |
| Datos relacionales | PostgreSQL o MySQL: votantes, elecciones, candidatos, marca de "ya votó" |
| Datos NoSQL | MongoDB o Redis: votos emitidos, log de auditoría, conteos |
| Conectores HTTP | Dos estilos distintos, por ejemplo REST entre servicios y GraphQL para consultar resultados desde el front |
| 3 lenguajes | Por ejemplo Java, Python y Node.js/TypeScript |
| Contenedores | Un `docker-compose.yml` que levante todo con un solo comando |

## Elementos y relaciones

### Componentes

- **Front-end web** (presentación): pantallas de login, votación y resultados.
- **Election Service** (lógica, Java): autenticación, elecciones y candidatos, y control de "ya votó".
- **Vote Service** (lógica, Python): recibe el voto, valida que el votante pueda votar y lo registra.
- **Results Service** (lógica, Node.js): calcula y expone el conteo.
- **PostgreSQL** (datos relacionales): votantes, elecciones, candidatos y la marca de "ya votó".
- **MongoDB** (datos NoSQL): votos sin vínculo al votante y registro de auditoría.

### Conectores

- **REST (HTTP):**
  - Front-end → Election Service (login y elecciones).
  - Front-end → Vote Service (emitir voto).
  - Vote Service → Election Service (validar y marcar "ya votó").
- **GraphQL (HTTP):** Front-end → Results Service para consultar resultados.
- **Acceso a datos (JDBC y driver de MongoDB):** no son HTTP, así que no cuentan para el requisito de conectores, pero conviene dibujarlos.

### Estilos arquitectónicos para la descripción

- **Cliente-servidor:** el front-end consume servicios remotos.
- **Tres capas (presentación, lógica, datos):** es lo que evidencian los colores.
- **Orientado a servicios:** tres servicios independientes, cada uno en su contenedor y con su lenguaje, que se comunican por HTTP.
- **Repositorio/datos compartidos:** los servicios acceden a bases de datos centrales.

### Puntos a tener en cuenta

- **Base de datos compartida:** Vote Service y Results Service usan el mismo MongoDB. En un prototipo es aceptable, pero se debe mencionar que a futuro se podría separar la lectura de la escritura (por ejemplo, con colecciones distintas).
- **Consistencia:** si Vote Service marca "ya votó" y luego falla al guardar el voto, el votante queda sin voto. Hay que documentar cómo se maneja, por ejemplo guardando primero el voto y marcando después, o con un reintento.
- **Verificación con el profesor:** confirmar que REST y GraphQL cuentan como dos tipos distintos de conectores HTTP, y si el JavaScript del front-end cuenta como uno de los tres lenguajes. Con esta propuesta no importa, porque el backend ya tiene tres (Java, Python y Node.js).
- **Formato del entregable:** el documento es `.md` exportado a PDF, así que el diagrama tendrá que ser una imagen (PNG o SVG) o un bloque Mermaid, según cómo se exporte.

## Decisiones de diseño

- **Separar identidad y voto:** el voto guardado en NoSQL no debe enlazarse al votante, y la marca "ya votó" vive en la base relacional. Esto justifica el uso de dos tipos de base de datos por razones arquitectónicas y no solo para cumplir el requisito.
- **Comunicación entre servicios:** el servicio de votos consulta al de elecciones (vía REST) para validar que el votante puede votar antes de registrar el voto. Así, el diagrama C&C muestra una interacción real entre los dos componentes de lógica.
- **Lenguajes:** un front-end en JavaScript podría o no contar como uno de los tres, según la interpretación del profesor. Para no arriesgarse, conviene que los tres lenguajes estén en el backend, o confirmarlo con él.
- **Conectores:** WebSocket no es estrictamente HTTP después del upgrade. REST + GraphQL es la combinación más segura, y conviene confirmarla con el profesor.
