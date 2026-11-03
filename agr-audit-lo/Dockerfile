# agr-audit-lo: compila con el SDK de .NET y corre solo con el runtime de ASP.NET.
FROM mcr.microsoft.com/dotnet/sdk:10.0-alpine AS build
WORKDIR /src

COPY AgrAuditLo.csproj .
RUN dotnet restore AgrAuditLo.csproj

COPY . .
RUN dotnet publish AgrAuditLo.csproj -c Release -o /app --no-restore


FROM mcr.microsoft.com/dotnet/aspnet:10.0-alpine
WORKDIR /app
COPY --from=build /app .

# Escucha en todas las interfaces (IPv4 e IPv6) en el puerto 8005.
ENV ASPNETCORE_HTTP_PORTS=8005
EXPOSE 8005
# Usuario sin privilegios que traen las imágenes de .NET.
USER $APP_UID

ENTRYPOINT ["dotnet", "AgrAuditLo.dll"]
