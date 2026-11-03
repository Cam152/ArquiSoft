FROM eclipse-temurin:21-jdk-alpine AS build
WORKDIR /app

# Instalar Maven en la imagen de compilación
RUN apk add --no-cache maven

COPY pom.xml .
COPY src ./src

# Compilar y empaquetar el .jar
RUN mvn clean package -DskipTests

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=build /app/target/*.jar app.jar
EXPOSE 8081
ENTRYPOINT ["java", "-jar", "app.jar"]