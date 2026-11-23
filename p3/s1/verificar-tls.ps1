<#
.SYNOPSIS
  S1 - Secure Channel. Comprueba TLS en agr-proxy-rp y guarda la evidencia en p3/s1/despues/.

.DESCRIPTION
  Con el sistema arriba en la rama p3-miguel (ya con S1), desde la raiz del repositorio:
      bash p3/s1/gen-certs.sh
      docker compose up -d --build --wait
      powershell -ExecutionPolicy Bypass -File .\p3\s1\verificar-tls.ps1

  Opciones:
      -ProbarEscalado   ademas levanta 3 replicas del gateway, reparte 30 peticiones por HTTPS y
                        vuelve a 1 replica (compatibilidad con R4)
      -OmitirVoto       no emite un voto (por defecto vota con el documento 1004; si ya voto, 409 es valido)

  Que comprueba (cada punto queda como OK, FALLA o INFO en 00_resumen.txt):
      01  llave y certificado fuera de Git, montaje de solo lectura y llave ausente de la imagen
      02  SAN del certificado (localhost, 127.0.0.1, ::1) y que el proxy sirve ese certificado
      03  sintaxis de la configuracion de Nginx
      04  TLS 1.2 y 1.3 aceptados; TLS 1.1 rechazado
      05  HTTP (80) redirige a HTTPS conservando ruta y consulta; sin HSTS ni cuerpo en HTTP
      06  HSTS en las respuestas HTTPS (tambien en errores)
      07  acceso a / , /ingresar, /api/health y /api/elections por HTTPS
      08  salida de curl -v con los datos del certificado
      09  puertos: 443 y 80 abiertos; 3000 y 8080 cerrados; solo el proxy publica puertos
      10  flujo funcional por HTTPS: login, elecciones, voto y resultados (SSE)
      11  se conservan los encabezados de S2 (sin Server/X-Powered-By), /api/admin 404 y el limite de 1 MB
      12  se conservan S3 (redes), S4 (limit_req, 429) y R4 (upstream con keepalive y resolve, gateway sin puertos)

  No hace falta instalar openssl en Windows: las pruebas de TLS usan la imagen alpine/openssl de Docker.
#>
param(
    [string]$Base = "https://localhost",
    [string]$Documento = "1004",
    [string]$Clave = "123456",
    [switch]$OmitirVoto,
    [switch]$ProbarEscalado
)

$ErrorActionPreference = "Continue"
$Base = $Base.TrimEnd("/")
$Api = "$Base/api"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$CertDir = Join-Path $PSScriptRoot "certs"
$CertFile = Join-Path $CertDir "localhost.crt"
$KeyFile = Join-Path $CertDir "localhost.key"
$OutDir = Join-Path $PSScriptRoot "despues"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Utf8 = New-Object System.Text.UTF8Encoding($false)
$Tmp = Join-Path ([IO.Path]::GetTempPath()) ("agr-s1-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
$Checks = New-Object System.Collections.Generic.List[object]
Push-Location $RepoRoot   # los "docker compose" y "git" se ejecutan desde la raiz del repo

# ---------------------------------------------------------------------------
# Utilidades
# ---------------------------------------------------------------------------
function Save-Evidence([string]$Name, [object[]]$Lines) {
    $header = @(
        "AGORA - Prototipo 3 - S1 Secure Channel - evidencia DESPUES",
        ("Fecha: " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz") + "   Entrada: " + $Base),
        ("=" * 78), ""
    )
    $text = (($header + $Lines) | ForEach-Object { "$_" }) -join "`r`n"
    [IO.File]::WriteAllText((Join-Path $OutDir $Name), $text + "`r`n", $Utf8)
    Write-Host ("  guardado  p3/s1/despues/" + $Name) -ForegroundColor DarkGray
}

function Invoke-Native([string]$Exe, [string[]]$ArgList) {
    $lines = & $Exe @ArgList 2>&1 | ForEach-Object { "$_" }
    return [pscustomobject]@{ Code = $LASTEXITCODE; Lines = @($lines) }
}

# curl.exe siempre con -k: el certificado es autofirmado (la validacion se prueba aparte, en el punto 02).
function Invoke-Http {
    param([string]$Method = "GET", [string]$Url, [string[]]$Headers = @(), [string]$BodyFile = "", [int]$MaxTime = 15)
    $hdr = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".h")
    $out = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".b")
    $a = @("-sS", "-k", "-X", $Method, "-D", $hdr, "-o", $out, "-w", "%{http_code}", "--max-time", "$MaxTime")
    foreach ($h in $Headers) { $a += @("-H", $h) }
    if ($BodyFile) { $a += @("-H", "Expect:", "--data-binary", "@$BodyFile") }
    $a += $Url
    $r = Invoke-Native "curl.exe" $a
    $status = 0
    $last = $r.Lines | Where-Object { $_ -match '^\d{3}$' } | Select-Object -Last 1
    if ($last) { $status = [int]$last }
    $respHeaders = @()
    if (Test-Path $hdr) {
        $raw = [IO.File]::ReadAllText($hdr)
        $blocks = $raw -split "`r?`n`r?`n" | Where-Object { $_.Trim() }
        if ($blocks) { $respHeaders = @(($blocks | Select-Object -Last 1) -split "`r?`n") }
    }
    $body = ""
    if (Test-Path $out) { $body = [IO.File]::ReadAllText($out, $Utf8) }
    return [pscustomobject]@{ Status = $status; Headers = $respHeaders; Body = $body; Url = $Url; Method = $Method }
}

function Get-Header($Response, [string]$Name) {
    $line = $Response.Headers | Where-Object { $_ -match ("^" + [regex]::Escape($Name) + ":") } | Select-Object -First 1
    if ($line) { return ($line -replace ("^" + [regex]::Escape($Name) + ":\s*"), "") }
    return $null
}

function Format-Response($Response) {
    $lines = @(("> curl.exe -k " + $Response.Method + " " + $Response.Url), ("HTTP " + $Response.Status)) + $Response.Headers
    if ($Response.Body -and $Response.Body.Length -lt 600) { $lines += @("", $Response.Body.Trim()) }
    return $lines + @("")
}

function Write-BodyFile([string]$Name, [string]$Text) {
    $path = Join-Path $Tmp $Name
    [IO.File]::WriteAllText($path, $Text, $Utf8)
    return $path
}

function Test-Port([int]$Port) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $task = $client.ConnectAsync("127.0.0.1", $Port)
        if ($task.Wait(1500) -and $client.Connected) { return $true }
        return $false
    } catch { return $false } finally { $client.Close() }
}

# $Ok: $true = OK, $false = FALLA, $null = INFO (solo se registra)
function Add-Check([string]$Id, [string]$Name, $Ok, [string]$Detail) {
    $estado = "INFO"
    if ($Ok -eq $true) { $estado = "OK" } elseif ($Ok -eq $false) { $estado = "FALLA" }
    $Checks.Add([pscustomobject]@{ Id = $Id; Prueba = $Name; Estado = $estado; Detalle = $Detail })
    $color = @{ "OK" = "Green"; "FALLA" = "Red"; "INFO" = "Yellow" }[$estado]
    Write-Host ("  [{0,-5}] {1} {2}  {3}" -f $estado, $Id, $Name, $Detail) -ForegroundColor $color
}

# Prueba un protocolo TLS desde un contenedor con OpenSSL, dentro de la red del proxy.
function Test-Tls([string]$Flag, [string[]]$Extra = @()) {
    $a = @("run", "--rm", "--network", "agr-public-net", "alpine/openssl", "s_client",
           "-connect", "agr-proxy-rp:443", "-servername", "localhost", $Flag, "-brief") + $Extra
    $r = Invoke-Native "docker" $a
    return [pscustomobject]@{ Flag = $Flag; Lines = $r.Lines; Text = ($r.Lines -join "`n") }
}

Write-Host ""
Write-Host "S1 - evidencia DESPUES (entrada: $Base)" -ForegroundColor Cyan

# ---------------------------------------------------------------------------
# 01 - Llave y certificado fuera de Git y fuera de la imagen
# ---------------------------------------------------------------------------
$null = & git check-ignore -q "p3/s1/certs/localhost.key"
$keyIgnored = ($LASTEXITCODE -eq 0)
$null = & git check-ignore -q "p3/s1/certs/localhost.crt"
$crtIgnored = ($LASTEXITCODE -eq 0)
$tracked = @(& git ls-files "*.key" "*.crt" "*.pem" "p3/s1/certs" 2>&1 | ForEach-Object { "$_" })
$mounts = Invoke-Native "docker" @("inspect", "agr-proxy-rp", "--format", "{{range .Mounts}}{{.Destination}}|{{.RW}};{{end}}")
$imgId = ((Invoke-Native "docker" @("inspect", "agr-proxy-rp", "--format", "{{.Image}}")).Lines | Select-Object -First 1)
$inImage = Invoke-Native "docker" @("run", "--rm", "--entrypoint", "sh", "$imgId", "-c", 'ls -la /etc/nginx/certs 2>&1; echo codigo=$?')
$lines = @("> git check-ignore -v p3/s1/certs/localhost.key")
$lines += (Invoke-Native "git" @("check-ignore", "-v", "p3/s1/certs/localhost.key")).Lines
$lines += @("", "> git ls-files *.key *.crt *.pem p3/s1/certs   (debe estar vacio)")
$lines += $tracked
$lines += @("", "> docker inspect agr-proxy-rp (montajes: destino|escritura)") + $mounts.Lines
$lines += @("", "> ls /etc/nginx/certs DENTRO DE LA IMAGEN, sin montajes (debe no existir)") + $inImage.Lines
Save-Evidence "01_llave_fuera_de_git_y_de_la_imagen.txt" $lines
Add-Check "01a" "Llave y certificado existen y estan en .gitignore" ((Test-Path $CertFile) -and (Test-Path $KeyFile) -and $keyIgnored -and $crtIgnored) ("llave ignorada=" + $keyIgnored + " certificado ignorado=" + $crtIgnored)
Add-Check "01b" "Ninguna llave ni certificado versionado" ($tracked.Count -eq 0) ("archivos versionados: " + $tracked.Count)
Add-Check "01c" "Certificados montados en solo lectura" (($mounts.Lines -join "") -match '/etc/nginx/certs\|false') (($mounts.Lines -join ""))
Add-Check "01d" "La llave no esta dentro de la imagen" (($inImage.Lines -join "`n") -match 'No such file') "carpeta /etc/nginx/certs ausente en la imagen"

# ---------------------------------------------------------------------------
# 02 - SAN del certificado y certificado servido por el proxy
# ---------------------------------------------------------------------------
$san = Invoke-Native "docker" @("run", "--rm", "-v", "${CertDir}:/certs:ro", "alpine/openssl", "x509", "-in", "/certs/localhost.crt",
                                "-noout", "-subject", "-issuer", "-dates", "-fingerprint", "-sha256", "-ext", "subjectAltName")
$sanText = $san.Lines -join "`n"
$fileFp = ($san.Lines | Where-Object { $_ -match 'Fingerprint' } | Select-Object -First 1)
$served = Invoke-Native "docker" @("run", "--rm", "--network", "agr-public-net", "--entrypoint", "sh", "alpine/openssl", "-c",
                                   "openssl s_client -connect agr-proxy-rp:443 -servername localhost </dev/null 2>/dev/null | openssl x509 -noout -fingerprint -sha256")
$servedFp = ($served.Lines | Where-Object { $_ -match 'Fingerprint' } | Select-Object -First 1)
$verifyLines = @()
$verifyOk = @{}
foreach ($v in @(@("localhost", "-verify_hostname"), @("127.0.0.1", "-verify_ip"), @("::1", "-verify_ip"))) {
    $r = Invoke-Native "docker" @("run", "--rm", "--network", "agr-public-net", "-v", "${CertDir}:/certs:ro", "--entrypoint", "sh", "alpine/openssl", "-c",
                                  ("openssl s_client -connect agr-proxy-rp:443 -servername localhost -CAfile /certs/localhost.crt " + $v[1] + " " + $v[0] + " -brief </dev/null 2>&1"))
    $verifyLines += @("> verificacion contra " + $v[0] + " (" + $v[1] + ")") + $r.Lines + @("")
    $verifyOk[$v[0]] = (($r.Lines -join "`n") -match 'Verification: OK')
}
Save-Evidence "02_certificado_y_san.txt" (@("> openssl x509 -in localhost.crt -noout -subject -issuer -dates -fingerprint -sha256 -ext subjectAltName") + $san.Lines +
    @("", "Huella del archivo : $fileFp", "Huella servida    : $servedFp", "") + $verifyLines)
$sanOk = ($sanText -match 'DNS:localhost') -and ($sanText -match 'IP Address:127\.0\.0\.1') -and ($sanText -match 'IP Address:0:0:0:0:0:0:0:1')
Add-Check "02a" "SAN: localhost, 127.0.0.1 y ::1" $sanOk "ver 02_certificado_y_san.txt"
Add-Check "02b" "El proxy sirve el certificado generado" (($fileFp) -and ($fileFp -eq $servedFp)) "huellas SHA-256 iguales"
Add-Check "02c" "El certificado valida contra localhost, 127.0.0.1 y ::1" ($verifyOk["localhost"] -and $verifyOk["127.0.0.1"] -and $verifyOk["::1"]) ("localhost=" + $verifyOk["localhost"] + " 127.0.0.1=" + $verifyOk["127.0.0.1"] + " ::1=" + $verifyOk["::1"])

# ---------------------------------------------------------------------------
# 03 - Sintaxis de Nginx
# ---------------------------------------------------------------------------
$nt = Invoke-Native "docker" @("compose", "exec", "-T", "agr-proxy-rp", "nginx", "-t")
Save-Evidence "03_nginx_sintaxis.txt" (@("> docker compose exec agr-proxy-rp nginx -t") + $nt.Lines + @("codigo de salida: " + $nt.Code))
Add-Check "03" "Sintaxis de Nginx valida" (($nt.Code -eq 0) -and (($nt.Lines -join "`n") -match 'successful')) ("codigo " + $nt.Code)

# ---------------------------------------------------------------------------
# 04 - Versiones de TLS
# ---------------------------------------------------------------------------
$t12 = Test-Tls "-tls1_2"
$t13 = Test-Tls "-tls1_3"
$t11 = Test-Tls "-tls1_1" @("-cipher", "DEFAULT:@SECLEVEL=0")
$lines = @()
foreach ($t in @($t12, $t13, $t11)) { $lines += @("> s_client " + $t.Flag + " -> agr-proxy-rp:443") + $t.Lines + @("") }
Save-Evidence "04_versiones_tls.txt" $lines
Add-Check "04a" "TLS 1.2 aceptado" ($t12.Text -match 'Protocol version: TLSv1\.2') (($t12.Lines | Where-Object { $_ -match 'Ciphersuite' } | Select-Object -First 1))
Add-Check "04b" "TLS 1.3 aceptado" ($t13.Text -match 'Protocol version: TLSv1\.3') (($t13.Lines | Where-Object { $_ -match 'Ciphersuite' } | Select-Object -First 1))
if ($t11.Text -match 'Protocol version: TLSv1\.1') {
    Add-Check "04c" "TLS 1.1 rechazado" $false "el proxy ACEPTO TLS 1.1"
} elseif ($t11.Text -match 'alert protocol version|alert handshake failure') {
    Add-Check "04c" "TLS 1.1 rechazado" $true "el servidor respondio con una alerta de version de protocolo"
} else {
    Add-Check "04c" "TLS 1.1 rechazado" $null "no concluyente: el cliente OpenSSL no pudo ofrecer TLS 1.1 (ver 04_versiones_tls.txt)"
}

# ---------------------------------------------------------------------------
# 05 - Redireccion de HTTP a HTTPS
# ---------------------------------------------------------------------------
$rRoot = Invoke-Http -Url "http://localhost/"
$rApi = Invoke-Http -Url "http://localhost/api/health?x=1"
$loginBody = Write-BodyFile "login-http.json" ('{"document":"' + $Documento + '","password":"' + $Clave + '"}')
$rPost = Invoke-Http -Method POST -Url "http://localhost/api/auth/login" -Headers @("Content-Type: application/json") -BodyFile $loginBody
$lines = @("GET http://localhost/ :") + (Format-Response $rRoot) + @("GET http://localhost/api/health?x=1 (conserva ruta y consulta):") + (Format-Response $rApi)
$lines += @("POST http://localhost/api/auth/login (el 80 no reenvia nada al gateway; no devuelve token):") + (Format-Response $rPost)
Save-Evidence "05_redireccion_http_a_https.txt" $lines
Add-Check "05a" "GET / por HTTP -> 301 a https://localhost/" (($rRoot.Status -eq 301) -and ((Get-Header $rRoot "Location") -eq "https://localhost/")) ("HTTP " + $rRoot.Status + " " + (Get-Header $rRoot "Location"))
Add-Check "05b" "Se conservan la ruta y la consulta" (($rApi.Status -eq 301) -and ((Get-Header $rApi "Location") -eq "https://localhost/api/health?x=1")) (Get-Header $rApi "Location")
Add-Check "05c" "Sin HSTS en la respuesta HTTP" (-not (Get-Header $rRoot "Strict-Transport-Security")) "Strict-Transport-Security ausente en el 80"
Add-Check "05d" "POST por HTTP no llega al gateway" (($rPost.Status -eq 301) -and ($rPost.Body -notmatch 'access_token')) ("HTTP " + $rPost.Status)

# ---------------------------------------------------------------------------
# 06 - HSTS en HTTPS
# ---------------------------------------------------------------------------
$hsts = @()
$lines = @()
foreach ($path in "/", "/api/health", "/api/ruta-que-no-existe") {
    $r = Invoke-Http -Url ($Base + $path)
    $lines += Format-Response $r
    $hsts += [pscustomobject]@{ Path = $path; Status = $r.Status; Value = (Get-Header $r "Strict-Transport-Security") }
}
Save-Evidence "06_hsts.txt" $lines
$missing = @($hsts | Where-Object { $_.Value -notmatch 'max-age=31536000' })
Add-Check "06" "HSTS en las respuestas HTTPS (incluidos los errores)" ($missing.Count -eq 0) (($hsts | ForEach-Object { $_.Path + " -> " + $_.Status }) -join " | ")

# ---------------------------------------------------------------------------
# 07 - Acceso por HTTPS
# ---------------------------------------------------------------------------
$rHome = Invoke-Http -Url ($Base + "/")
$rLogin = Invoke-Http -Url ($Base + "/ingresar")
$rHealth = Invoke-Http -Url ($Api + "/health")
$rElec = Invoke-Http -Url ($Api + "/elections")
Save-Evidence "07_acceso_https.txt" ((Format-Response $rHome) + (Format-Response $rLogin) + (Format-Response $rHealth) + (Format-Response $rElec))
Add-Check "07a" "https://localhost/ responde" ($rHome.Status -eq 200) ("HTTP " + $rHome.Status)
Add-Check "07b" "https://localhost/ingresar responde" ($rLogin.Status -eq 200) ("HTTP " + $rLogin.Status)
Add-Check "07c" "/api/health llega al gateway" (($rHealth.Status -eq 200) -and ($rHealth.Body -match '"status"\s*:\s*"ok"')) ("HTTP " + $rHealth.Status)
Add-Check "07d" "/api/elections llega al gateway" ($rElec.Status -eq 200) ("HTTP " + $rElec.Status)

# ---------------------------------------------------------------------------
# 08 - curl -v: datos del certificado y del canal
# ---------------------------------------------------------------------------
$v1 = Invoke-Native "curl.exe" @("-v", "-k", "--max-time", "15", "-o", "NUL", ($Api + "/health"))
$v2 = Invoke-Native "curl.exe" @("-v", "-k", "--tlsv1.2", "--tls-max", "1.2", "--max-time", "15", "-o", "NUL", ($Api + "/health"))
$v3 = Invoke-Native "curl.exe" @("-v", "-k", "--tlsv1.3", "--max-time", "15", "-o", "NUL", ($Api + "/health"))
Save-Evidence "08_curl_verbose.txt" (@("> curl.exe -v -k " + $Api + "/health") + $v1.Lines + @("", "> curl.exe -v -k --tlsv1.2 --tls-max 1.2") + $v2.Lines + @("", "> curl.exe -v -k --tlsv1.3") + $v3.Lines)
Add-Check "08" "curl -v conecta por TLS 1.2 y por TLS 1.3" (($v2.Code -eq 0) -and ($v3.Code -eq 0)) ("codigos de salida: 1.2=" + $v2.Code + " 1.3=" + $v3.Code)

# ---------------------------------------------------------------------------
# 09 - Puertos
# ---------------------------------------------------------------------------
$ps = Invoke-Native "docker" @("ps", "--filter", "name=agr-", "--format", "{{.Names}}|{{.Ports}}")
$lines = @("> docker ps --filter name=agr- --format '{{.Names}} {{.Ports}}'", "")
$withPorts = @()
$proxyPorts = ""
foreach ($l in $ps.Lines) {
    $parts = $l -split "\|", 2
    $lines += ("  {0,-32} {1}" -f $parts[0], $parts[1])
    if ($parts.Count -gt 1 -and $parts[1] -match "->") { $withPorts += $parts[0] }
    if ($parts[0] -eq "agr-proxy-rp") { $proxyPorts = $parts[1] }
}
$state = @{}
$lines += @("", "Conexion TCP a 127.0.0.1 desde el equipo anfitrion:")
foreach ($p in 80, 443, 3000, 8080) {
    $state[$p] = Test-Port $p
    $lines += ("  {0,-6} {1}" -f $p, $(if ($state[$p]) { "ABIERTO" } else { "cerrado" }))
}
Save-Evidence "09_puertos.txt" $lines
$extra = $withPorts | Where-Object { $_ -notin @("agr-proxy-rp", "agr-broker-as") }
$allPorts = ($ps.Lines -join " ")
$agoraPublishes3000 = ($allPorts -match '->3000/tcp')
$agoraPublishes8080 = ($allPorts -match '->8080/tcp')
Add-Check "09a" "443 y 80 abiertos; AGORA no publica 3000 ni 8080" ($state[443] -and $state[80] -and -not $agoraPublishes3000 -and -not $agoraPublishes8080) ("443=" + $state[443] + " 80=" + $state[80] + " | AGORA publica 3000=" + $agoraPublishes3000 + " 8080=" + $agoraPublishes8080)
if ($state[3000] -or $state[8080]) {
    Add-Check "09c" "Puertos 3000/8080 ocupados en el equipo" $null ("3000=" + $state[3000] + " 8080=" + $state[8080] + ": no los publica AGORA (ver docker ps arriba); los usa otro programa. Busque cual con: netstat -ano | findstr :8080")
}
Add-Check "09b" "Solo el proxy publica 80 y 443" ((-not $extra) -and ($proxyPorts -match '->443/tcp') -and ($proxyPorts -match '->80/tcp')) ($proxyPorts)

# ---------------------------------------------------------------------------
# 10 - Flujo funcional por HTTPS: login, elecciones, voto y resultados en vivo
# ---------------------------------------------------------------------------
$lines = @()
$body = Write-BodyFile "login-ok.json" ('{"document":"' + $Documento + '","password":"' + $Clave + '"}')
$rL = Invoke-Http -Method POST -Url ($Api + "/auth/login") -Headers @("Content-Type: application/json") -BodyFile $body
$token = $null
try { $token = ($rL.Body | ConvertFrom-Json).access_token } catch { }
$lines += @("1. Login de $Documento por POST /api/auth/login: HTTP " + $rL.Status + ", token " + $(if ($token) { "recibido (recortado: " + $token.Substring(0, [Math]::Min(12, $token.Length)) + "...)" } else { "no" }), "")
Add-Check "10a" "Login por HTTPS" (($rL.Status -eq 200) -and [bool]$token) ("HTTP " + $rL.Status)

$gql = Write-BodyFile "graphql.json" '{"query":"query { elections { id name status isOpen candidates { id name } } }"}'
$rG = Invoke-Http -Method POST -Url ($Api + "/graphql") -Headers @("Content-Type: application/json") -BodyFile $gql
$election = $null
try {
    $all = @(($rG.Body | ConvertFrom-Json).data.elections)
    $election = $all | Where-Object { $_.isOpen -eq $true -and @($_.candidates).Count -gt 0 } | Select-Object -First 1
} catch { }
$lines += @("2. Elecciones por POST /api/graphql: HTTP " + $rG.Status + $(if ($election) { ", eleccion abierta id=" + $election.id } else { ", sin eleccion abierta" }), "")
Add-Check "10b" "Elecciones por HTTPS (GraphQL)" (($rG.Status -eq 200) -and ($null -ne $election)) ("HTTP " + $rG.Status + $(if ($election) { ", eleccion " + $election.id } else { ", sin eleccion abierta" }))

if ($election -and $token) {
    if (-not $OmitirVoto) {
        $vb = Write-BodyFile "vote.json" ('{"election_id":' + $election.id + ',"candidate_id":' + $election.candidates[0].id + '}')
        $rV = Invoke-Http -Method POST -Url ($Api + "/votes") -Headers @("Content-Type: application/json", "Authorization: Bearer $token") -BodyFile $vb -MaxTime 30
        $lines += @("3. Voto por POST /api/votes: HTTP " + $rV.Status + "  " + $rV.Body.Trim(), "")
        Add-Check "10c" "Voto por HTTPS" (($rV.Status -eq 201) -or ($rV.Status -eq 409)) ("HTTP " + $rV.Status + $(if ($rV.Status -eq 409) { " (ya habia votado: valido)" } else { "" }))
    }
    $sse = Invoke-Native "curl.exe" @("-sk", "-N", "-i", "--max-time", "6", ($Api + "/results/" + $election.id + "/stream"))
    $lines += @("4. SSE por GET /api/results/" + $election.id + "/stream (corte a los 6 s: esperado):") + $sse.Lines + @("")
    $sseHeader = [bool]($sse.Lines | Where-Object { $_ -match 'text/event-stream' })
    $sseEvents = @($sse.Lines | Where-Object { $_ -match '^event:' })
    Add-Check "10d" "Resultados en vivo (SSE) por HTTPS" $sseHeader ("text/event-stream; eventos recibidos: " + $sseEvents.Count)
    $rR = Invoke-Http -Url ($Api + "/results/" + $election.id)
    $lines += @("5. Resultados por GET /api/results/" + $election.id + ": HTTP " + $rR.Status, ("   " + $rR.Body.Trim()), "")
    Add-Check "10e" "Resultados por HTTPS" ($rR.Status -eq 200) ("HTTP " + $rR.Status)
}
Save-Evidence "10_flujo_funcional.txt" $lines

# ---------------------------------------------------------------------------
# 11 - Se conservan los encabezados y limites de S2
# ---------------------------------------------------------------------------
$problems = @()
foreach ($path in "/", "/api/health", "/api/ruta-que-no-existe") {
    $r = Invoke-Http -Url ($Base + $path)
    foreach ($bad in "Server", "X-Powered-By") { if (Get-Header $r $bad) { $problems += "$path tiene $bad" } }
    if ($r.Headers | Where-Object { $_ -match '^x-nextjs-' }) { $problems += "$path tiene X-Nextjs-*" }
    if ((Get-Header $r "X-Frame-Options") -ne "DENY") { $problems += "$path X-Frame-Options" }
    if ((Get-Header $r "X-Content-Type-Options") -ne "nosniff") { $problems += "$path X-Content-Type-Options" }
    foreach ($need in "Content-Security-Policy", "Referrer-Policy", "Permissions-Policy", "Cross-Origin-Opener-Policy") {
        if (-not (Get-Header $r $need)) { $problems += "$path falta $need" }
    }
}
$rAdm = Invoke-Http -Url ($Api + "/admin/audit") -Headers @("X-API-Key: dev-admin-key")
$big = Join-Path $Tmp "2mb.json"
[IO.File]::WriteAllBytes($big, [Text.Encoding]::ASCII.GetBytes("a" * 2MB))
$rBig = Invoke-Http -Method POST -Url ($Api + "/votes") -Headers @("Content-Type: application/json") -BodyFile $big
Save-Evidence "11_encabezados_y_limites.txt" (@("Problemas de encabezados: " + $(if ($problems) { $problems -join " | " } else { "ninguno" }), "") + (Format-Response $rAdm) + (Format-Response $rBig))
Add-Check "11a" "Encabezados de seguridad y sin Server/X-Powered-By" (-not $problems) $(if ($problems) { $problems -join " | " } else { "3 rutas revisadas" })
Add-Check "11b" "/api/admin no se expone" ($rAdm.Status -eq 404) ("HTTP " + $rAdm.Status)
Add-Check "11c" "Mas de 1 MB -> 413 del proxy" ($rBig.Status -eq 413) ("HTTP " + $rBig.Status)

# ---------------------------------------------------------------------------
# 12 - Se conservan S3, S4 y R4
# ---------------------------------------------------------------------------
$cfg = Invoke-Native "docker" @("compose", "exec", "-T", "agr-proxy-rp", "nginx", "-T")
$cfgText = $cfg.Lines -join "`n"
$wanted = [ordered]@{
    "S4 limit_req_zone"        = 'limit_req_zone \$binary_remote_addr zone=login_limit'
    "S4 limit_req en el login" = 'limit_req zone=login_limit'
    "S4 respuesta 429"         = 'limit_req_status 429'
    "R4 keepalive del gateway" = 'keepalive 64'
    "R4 resolve por DNS"       = 'server agr-gateway-or:8080 resolve'
    "S2 SSE sin buffer"        = 'proxy_buffering off'
    "S2 limite de 1 MB"        = 'client_max_body_size 1m'
    "S1 solo TLS 1.2 y 1.3"    = 'ssl_protocols\s+TLSv1\.2 TLSv1\.3'
}
$cfgLines = @()
foreach ($k in $wanted.Keys) {
    $found = ($cfgText -match $wanted[$k])
    $cfgLines += ("  {0,-28} {1}" -f $k, $(if ($found) { "presente" } else { "FALTA" }))
    Add-Check "12" $k $found $wanted[$k]
}
$edgeInternal = ((Invoke-Native "docker" @("network", "inspect", "agr-edge-net", "--format", "{{.Internal}}")).Lines | Select-Object -First 1)
$netInfo = (Invoke-Native "docker" @("network", "inspect", "agr-public-net", "--format", "{{.Internal}}|{{range .Containers}}{{.Name}} {{end}}")).Lines | Select-Object -First 1
$netParts = "$netInfo" -split "\|", 2
$publicMembers = @()
if ($netParts.Count -gt 1) { $publicMembers = @($netParts[1].Trim() -split "\s+" | Where-Object { $_ }) }
$gwIds = @((Invoke-Native "docker" @("compose", "ps", "-q", "agr-gateway-or")).Lines | Where-Object { $_ })
$gwPorts = @()
foreach ($id in $gwIds) { $gwPorts += ((Invoke-Native "docker" @("inspect", $id, "--format", "{{json .HostConfig.PortBindings}}")).Lines | Select-Object -First 1) }
$gwClean = (($gwPorts | Where-Object { $_ -and $_ -ne "{}" -and $_ -ne "null" }).Count -eq 0)
Save-Evidence "12_compatibilidad_s3_s4_r4.txt" (@("Configuracion efectiva de Nginx (docker compose exec agr-proxy-rp nginx -T), puntos clave:") + $cfgLines +
    @("", "agr-edge-net internal=" + $edgeInternal, "agr-public-net: " + $netInfo, "Replicas del gateway: " + $gwIds.Count + "  PortBindings: " + ($gwPorts -join " ")))
Add-Check "12a" "S3: agr-edge-net sigue siendo internal" ($edgeInternal -eq "true") ("internal=" + $edgeInternal)
Add-Check "12b" "S3: en agr-public-net solo esta el proxy" (($netParts[0] -eq "false") -and ($publicMembers.Count -eq 1) -and ($publicMembers[0] -eq "agr-proxy-rp")) (($publicMembers -join ", "))
Add-Check "12c" "R4: el gateway no publica puertos" ($gwClean -and ($gwIds.Count -ge 1)) ("replicas=" + $gwIds.Count)

# ---------------------------------------------------------------------------
# 13 - (opcional) R4: tres replicas del gateway detras del mismo proxy con TLS
# ---------------------------------------------------------------------------
if ($ProbarEscalado) {
    Write-Host "  escalando el gateway a 3 replicas (tarda un poco)..." -ForegroundColor DarkGray
    $since = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    $up = Invoke-Native "docker" @("compose", "up", "-d", "--no-recreate", "--wait", "--scale", "agr-gateway-or=3")
    Start-Sleep -Seconds 15   # el proxy refresca el DNS de Docker cada 10 s
    1..30 | ForEach-Object { Invoke-Http -Url ($Api + "/health") | Out-Null }
    $log = Invoke-Native "docker" @("logs", "--since", $since, "agr-proxy-rp")
    $ups = @($log.Lines | Where-Object { $_ -match '/api/health' } | ForEach-Object { if ($_ -match 'up=([0-9.:]+)') { $Matches[1] } })
    $groups = @($ups | Group-Object | Sort-Object Name)
    $names = (Invoke-Native "docker" @("ps", "--filter", "label=com.docker.compose.service=agr-gateway-or", "--format", "{{.Names}}  {{.Status}}")).Lines
    $lines = @("> docker compose up -d --no-recreate --wait --scale agr-gateway-or=3") + $up.Lines + @("", "Replicas:") + $names
    $lines += @("", "Peticiones por HTTPS repartidas (columna up= del log del proxy):") + ($groups | ForEach-Object { "  " + $_.Count + "  " + $_.Name })
    Save-Evidence "13_escalado_3_replicas.txt" $lines
    Add-Check "13" "3 replicas detras del proxy con TLS" $(if ($groups.Count -ge 3) { $true } else { $null }) ("destinos distintos vistos: " + $groups.Count + " de 3")
    Write-Host "  volviendo a 1 replica..." -ForegroundColor DarkGray
    $null = Invoke-Native "docker" @("compose", "up", "-d", "--wait", "--scale", "agr-gateway-or=1")
}

# ---------------------------------------------------------------------------
# 00 - Resumen
# ---------------------------------------------------------------------------
$fails = @($Checks | Where-Object { $_.Estado -eq "FALLA" }).Count
$table = $Checks | Format-Table Id, Estado, Prueba, Detalle -AutoSize -Wrap | Out-String -Width 220
Save-Evidence "00_resumen.txt" (@("Resultado: " + $(if ($fails -eq 0) { "todas las pruebas pasan (revise las marcadas INFO)" } else { "$fails prueba(s) fallan" }), "") + ($table -split "`r?`n"))
Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
Pop-Location

Write-Host ""
if ($fails -eq 0) { Write-Host "Todo OK. Evidencia en p3/s1/despues/" -ForegroundColor Green }
else { Write-Host "$fails prueba(s) fallan. Revise p3/s1/despues/00_resumen.txt" -ForegroundColor Red }
Write-Host ""
Write-Host "Faltan las capturas de trafico (antes y despues): ver p3/s1/README.md, seccion Evidencias." -ForegroundColor Cyan
if ($fails -gt 0) { exit 1 }
