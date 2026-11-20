<#
.SYNOPSIS
  S2 · Reverse Proxy. Evidencia DESPUÉS: prueba agr-proxy-rp de punta a punta y
  guarda los resultados en p3/s2/despues/.

.DESCRIPTION
  Se ejecuta con el sistema arriba en la rama p3/cristian (ya con S2):
      docker compose up -d --build --wait
      powershell -ExecutionPolicy Bypass -File .\p3\s2\verificar-proxy.ps1

  Cuando entre S1 (TLS), se repite contra HTTPS:
      powershell -ExecutionPolicy Bypass -File .\p3\s2\verificar-proxy.ps1 -Base https://localhost -Inseguro

  Qué comprueba (cada punto queda como OK, FALLA o INFO en 00_resumen.txt):
      01  solo agr-proxy-rp publica puertos (más la consola de RabbitMQ en 127.0.0.1)
      02  3000 y 8080 ya no aceptan conexión desde el equipo; 80 sí
      03  sin Server, X-Powered-By ni X-Nextjs-*; encabezados de seguridad presentes
      04  "/" va a la web, "/api/..." al gateway sin el prefijo; /api/admin no se expone
      05  más de 1 MB -> 413 del proxy (JSON); menos de 1 MB sí pasa
      06  flujo completo por el proxy: login, elecciones, voto y SSE en vivo
      07  límite de login por IP: 429 del proxy (y 423 del Auth Service si ya está S4)
      08  caída del gateway: 502 en JSON y recuperación sin reiniciar el proxy
      09  redes: agr-edge-net es internal y en agr-public-net solo está el proxy
      10  configuración efectiva de Nginx y últimas líneas de su log

  Cuidado: el punto 07 bloquea 15 minutos la cuenta de prueba (por defecto 1005).
  Al final el script muestra el comando para desbloquearla.
#>
param(
    [string]$Base = "http://localhost",
    [switch]$Inseguro,
    [string]$Documento = "1004",
    [string]$Clave = "123456",
    [string]$DocumentoBloqueo = "1005",
    [switch]$OmitirCaida
)

$ErrorActionPreference = "Continue"
$Base = $Base.TrimEnd("/")
$Api = "$Base/api"
$EntryPort = ([Uri]$Base).Port   # 80 con http, 443 con https (o el de PROXY_HTTP_PORT)
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$OutDir = Join-Path $PSScriptRoot "despues"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Utf8 = New-Object System.Text.UTF8Encoding($false)
$Tmp = Join-Path ([IO.Path]::GetTempPath()) ("agr-s2-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
$Tls = @()
if ($Inseguro) { $Tls = @("-k") }   # certificado autofirmado de S1
$Checks = New-Object System.Collections.Generic.List[object]
Push-Location $RepoRoot   # los "docker compose" se ejecutan desde la raíz del repo

# ---------------------------------------------------------------------------
# Utilidades
# ---------------------------------------------------------------------------
function Save-Evidence([string]$Name, [object[]]$Lines) {
    $header = @(
        "AGORA · Prototipo 3 · S2 Reverse Proxy · evidencia DESPUÉS",
        ("Fecha: " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz") + "   Entrada: " + $Base),
        ("=" * 78), ""
    )
    $text = (($header + $Lines) | ForEach-Object { "$_" }) -join "`r`n"
    [IO.File]::WriteAllText((Join-Path $OutDir $Name), $text + "`r`n", $Utf8)
    Write-Host ("  guardado  p3/s2/despues/" + $Name) -ForegroundColor DarkGray
}

function Invoke-Native([string]$Exe, [string[]]$ArgList) {
    $lines = & $Exe @ArgList 2>&1 | ForEach-Object { "$_" }
    return [pscustomobject]@{ Code = $LASTEXITCODE; Lines = @($lines) }
}

function Invoke-Http {
    param([string]$Method = "GET", [string]$Url, [string[]]$Headers = @(), [string]$BodyFile = "", [int]$MaxTime = 15)
    $hdr = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".h")
    $out = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".b")
    $a = @("-sS", "-X", $Method, "-D", $hdr, "-o", $out, "-w", "%{http_code}", "--max-time", "$MaxTime") + $Tls
    foreach ($h in $Headers) { $a += @("-H", $h) }
    if ($BodyFile) { $a += @("-H", "Expect:", "--data-binary", "@$BodyFile") }
    $a += $Url
    $r = Invoke-Native "curl.exe" $a
    $status = 0
    $last = $r.Lines | Where-Object { $_ -match '^\d{3}$' } | Select-Object -Last 1
    if ($last) { $status = [int]$last }
    $headers = @()
    if (Test-Path $hdr) {
        $raw = [IO.File]::ReadAllText($hdr)
        $blocks = $raw -split "`r?`n`r?`n" | Where-Object { $_.Trim() }
        if ($blocks) { $headers = @(($blocks | Select-Object -Last 1) -split "`r?`n") }
    }
    $body = ""
    if (Test-Path $out) { $body = [IO.File]::ReadAllText($out, $Utf8) }
    return [pscustomobject]@{ Status = $status; Headers = $headers; Body = $body; Url = $Url; Method = $Method }
}

function Get-Header($Response, [string]$Name) {
    $line = $Response.Headers | Where-Object { $_ -match ("^" + [regex]::Escape($Name) + ":") } | Select-Object -First 1
    if ($line) { return ($line -replace ("^" + [regex]::Escape($Name) + ":\s*"), "") }
    return $null
}

function Count-Header($Response, [string]$Name) {
    return @($Response.Headers | Where-Object { $_ -match ("^" + [regex]::Escape($Name) + ":") }).Count
}

function Format-Response($Response) {
    $lines = @(("> curl.exe " + $Response.Method + " " + $Response.Url), ("HTTP " + $Response.Status)) + $Response.Headers
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

function Test-ProxyHeaders($Response) {
    $problems = @()
    foreach ($bad in "Server", "X-Powered-By") { if (Get-Header $Response $bad) { $problems += "tiene $bad" } }
    if ($Response.Headers | Where-Object { $_ -match '^x-nextjs-' }) { $problems += "tiene X-Nextjs-*" }
    if ((Get-Header $Response "X-Frame-Options") -ne "DENY" -or (Count-Header $Response "X-Frame-Options") -ne 1) { $problems += "X-Frame-Options" }
    if ((Get-Header $Response "X-Content-Type-Options") -ne "nosniff") { $problems += "X-Content-Type-Options" }
    foreach ($need in "Content-Security-Policy", "Referrer-Policy", "Permissions-Policy") {
        if (-not (Get-Header $Response $need)) { $problems += "falta $need" }
    }
    return $problems
}

Write-Host ""
Write-Host "S2 · evidencia DESPUÉS (entrada: $Base)" -ForegroundColor Cyan

# ---------------------------------------------------------------------------
# 01 · Puertos publicados por Docker
# ---------------------------------------------------------------------------
$ps = Invoke-Native "docker" @("ps", "--filter", "name=agr-", "--format", "{{.Names}}|{{.Ports}}")
$lines = @("> docker ps --filter name=agr- --format '{{.Names}} {{.Ports}}'", "")
$withPorts = @()
foreach ($l in $ps.Lines) {
    $parts = $l -split "\|", 2
    $lines += ("  {0,-18} {1}" -f $parts[0], $parts[1])
    if ($parts.Count -gt 1 -and $parts[1] -match "->") { $withPorts += $parts[0] }
}
Save-Evidence "01_puertos_publicados.txt" $lines
$extra = $withPorts | Where-Object { $_ -notin @("agr-proxy-rp", "agr-broker-as") }
$brokerLine = ($ps.Lines | Where-Object { $_ -like "agr-broker-as|*" }) -join ""
$brokerLocal = ($brokerLine -notmatch "0\.0\.0\.0:15672") -and ($brokerLine -notmatch ":::15672")
Add-Check "01" "Solo el proxy publica puertos" ((-not $extra) -and ($withPorts -contains "agr-proxy-rp") -and $brokerLocal) (($withPorts -join ", "))

# ---------------------------------------------------------------------------
# 02 · Puertos de localhost
# ---------------------------------------------------------------------------
$state = @{}
$lines = @("Conexión TCP a 127.0.0.1 desde el equipo anfitrión:", "")
foreach ($p in (@(80, 443, 3000, 8080, 15672, $EntryPort) | Select-Object -Unique)) {
    $state[$p] = Test-Port $p
    $lines += ("  {0,-6} {1}" -f $p, $(if ($state[$p]) { "ABIERTO" } else { "cerrado" }))
}
Save-Evidence "02_puertos_desde_host.txt" $lines
Add-Check "02" "3000 y 8080 cerrados, entrada abierta" ($state[$EntryPort] -and -not $state[3000] -and -not $state[8080]) ("$EntryPort=" + $state[$EntryPort] + " 3000=" + $state[3000] + " 8080=" + $state[8080])

# ---------------------------------------------------------------------------
# 03 · Encabezados: sin tecnología a la vista y con seguridad
# ---------------------------------------------------------------------------
$lines = @()
$allProblems = @()
foreach ($path in "/", "/ingresar", "/ruta-que-no-existe", "/api/health", "/api/ruta-que-no-existe") {
    $r = Invoke-Http -Url ($Base + $path)
    $lines += Format-Response $r
    $p = Test-ProxyHeaders $r
    if ($r.Status -eq 0) { $p += "sin respuesta" }
    if ($p) { $allProblems += ("$path -> " + ($p -join ", ")) }
}
$lines += @("Problemas encontrados: " + $(if ($allProblems) { $allProblems -join " | " } else { "ninguno" }))
Save-Evidence "03_encabezados.txt" $lines
Add-Check "03" "Sin Server/X-Powered-By, con encabezados de seguridad" (-not $allProblems) $(if ($allProblems) { $allProblems -join " | " } else { "5 rutas revisadas" })

# ---------------------------------------------------------------------------
# 04 · Enrutamiento
# ---------------------------------------------------------------------------
$rApiHealth = Invoke-Http -Url ($Api + "/health")
$rWebHealth = Invoke-Http -Url ($Base + "/health")
$rElections = Invoke-Http -Url ($Api + "/elections")
$rAdmin = Invoke-Http -Url ($Api + "/admin/audit") -Headers @("X-API-Key: dev-admin-key")
$rCors = Invoke-Http -Url ($Api + "/elections") -Headers @("Origin: https://sitio-ajeno.example")
$lines = @("/api/health llega al gateway como /health:") + (Format-Response $rApiHealth)
$lines += @("/health (sin /api) lo atiende la web, no el gateway:") + (Format-Response $rWebHealth)[0..1] + @("")
$lines += @("/api/elections llega al gateway como /elections:") + (Format-Response $rElections)[0..1] + @("")
$lines += @("/api/admin/... no se expone hacia fuera (la CLI usa la red interna):") + (Format-Response $rAdmin)
$lines += @("Un origen ajeno no recibe Access-Control-Allow-Origin (el navegador usa el mismo origen):") + (Format-Response $rCors)[0..1]
$lines += @("  Access-Control-Allow-Origin: " + $(if (Get-Header $rCors "Access-Control-Allow-Origin") { Get-Header $rCors "Access-Control-Allow-Origin" } else { "(ausente)" }), "")
Save-Evidence "04_enrutamiento.txt" $lines
Add-Check "04a" "/api/health -> gateway /health" (($rApiHealth.Status -eq 200) -and ($rApiHealth.Body -match '"status"\s*:\s*"ok"')) ("HTTP " + $rApiHealth.Status)
Add-Check "04b" "/health -> web (no es del gateway)" ($rWebHealth.Status -eq 404) ("HTTP " + $rWebHealth.Status)
Add-Check "04c" "/api/elections -> gateway /elections" ($rElections.Status -eq 200) ("HTTP " + $rElections.Status)
Add-Check "04d" "/api/admin no se expone" ($rAdmin.Status -eq 404) ("HTTP " + $rAdmin.Status)
Add-Check "04e" "Origen ajeno sin CORS" (-not (Get-Header $rCors "Access-Control-Allow-Origin")) "Access-Control-Allow-Origin ausente"

# ---------------------------------------------------------------------------
# 05 · Límite de 1 MB
# ---------------------------------------------------------------------------
$big = Join-Path $Tmp "2mb.json"
[IO.File]::WriteAllBytes($big, [Text.Encoding]::ASCII.GetBytes("a" * 2MB))
$small = Join-Path $Tmp "900kb.json"
[IO.File]::WriteAllBytes($small, [Text.Encoding]::ASCII.GetBytes("a" * 900KB))
$r2Api = Invoke-Http -Method POST -Url ($Api + "/votes") -Headers @("Content-Type: application/json") -BodyFile $big
$r2Web = Invoke-Http -Method POST -Url ($Base + "/") -Headers @("Content-Type: application/json") -BodyFile $big
$r900 = Invoke-Http -Method POST -Url ($Api + "/votes") -Headers @("Content-Type: application/json") -BodyFile $small
$lines = @("POST de 2 MB a /api/votes:") + (Format-Response $r2Api)
$lines += @("POST de 2 MB a / (web):") + (Format-Response $r2Web)
$lines += @("POST de 900 KB a /api/votes (pasa el proxy; el gateway responde por falta de token):") + (Format-Response $r900)
Save-Evidence "05_limite_1mb.txt" $lines
Add-Check "05a" "2 MB a /api -> 413 JSON del proxy" (($r2Api.Status -eq 413) -and ($r2Api.Body -match "1 MB")) ("HTTP " + $r2Api.Status)
Add-Check "05b" "2 MB a / -> 413" ($r2Web.Status -eq 413) ("HTTP " + $r2Web.Status)
Add-Check "05c" "900 KB sí pasa el proxy" (($r900.Status -ne 413) -and ($r900.Status -ne 0)) ("HTTP " + $r900.Status)

# ---------------------------------------------------------------------------
# 06 · Flujo completo por el proxy: login, elecciones, voto y SSE
# ---------------------------------------------------------------------------
$lines = @()
$loginBody = Write-BodyFile "login-ok.json" ('{"document":"' + $Documento + '","password":"' + $Clave + '"}')
$rLogin = Invoke-Http -Method POST -Url ($Api + "/auth/login") -Headers @("Content-Type: application/json") -BodyFile $loginBody
$token = $null
try { $token = ($rLogin.Body | ConvertFrom-Json).access_token } catch { }
$lines += @("1. Login de $Documento por POST /api/auth/login:", ("> curl.exe POST " + $rLogin.Url), ("HTTP " + $rLogin.Status))
$lines += ("   token recibido: " + $(if ($token) { $token.Substring(0, [Math]::Min(16, $token.Length)) + "... (recortado)" } else { "no" }))
$lines += ""
Add-Check "06a" "Login por /api/auth/login" (($rLogin.Status -eq 200) -and [bool]$token) ("HTTP " + $rLogin.Status)

$gqlBody = Write-BodyFile "graphql.json" '{"query":"query { elections { id name status isOpen candidates { id name } } }"}'
$rGql = Invoke-Http -Method POST -Url ($Api + "/graphql") -Headers @("Content-Type: application/json") -BodyFile $gqlBody
$election = $null
try {
    $all = @(($rGql.Body | ConvertFrom-Json).data.elections)
    $election = $all | Where-Object { $_.isOpen -eq $true -and @($_.candidates).Count -gt 0 } | Select-Object -First 1
} catch { }
$lines += @("2. Elecciones por POST /api/graphql: HTTP " + $rGql.Status)
if ($election) { $lines += ("   elección abierta: id=" + $election.id + " (" + $election.name + "), candidato " + $election.candidates[0].id) }
else { $lines += "   no hay elecciones abiertas con candidatos (actívenlas con la CLI y repitan)" }
$lines += ""
Add-Check "06b" "Elecciones por /api/graphql" (($rGql.Status -eq 200) -and ($null -ne $election)) ("HTTP " + $rGql.Status + $(if ($election) { ", elección " + $election.id } else { ", sin elección abierta" }))

if ($election -and $token) {
    $sseUrl = "$Api/results/$($election.id)/stream"
    $job = Start-Job -ArgumentList $sseUrl, $Inseguro.IsPresent -ScriptBlock {
        param($Url, $Insecure)
        $a = @("-sS", "-N", "-i", "--max-time", "18")
        if ($Insecure) { $a += "-k" }
        & curl.exe @a $Url 2>&1 | ForEach-Object { (Get-Date -Format "HH:mm:ss.fff") + " | " + "$_" }
    }
    Start-Sleep -Seconds 6   # tiempo para que el job arranque y llegue el primer evento
    $voteBody = Write-BodyFile "vote.json" ('{"election_id":' + $election.id + ',"candidate_id":' + $election.candidates[0].id + '}')
    $voteAt = Get-Date -Format "HH:mm:ss.fff"
    $rVote = Invoke-Http -Method POST -Url ($Api + "/votes") -Headers @("Content-Type: application/json", "Authorization: Bearer $token") -BodyFile $voteBody -MaxTime 30
    $lines += @("3. Voto por POST /api/votes a las $voteAt -> HTTP " + $rVote.Status + "  " + $rVote.Body.Trim(), "")
    Wait-Job $job -Timeout 30 | Out-Null
    $sseLines = @(Receive-Job $job)
    Remove-Job $job -Force
    $lines += @("4. SSE por GET $sseUrl (hora de llegada de cada línea; el corte a los 18 s es esperado):") + $sseLines + @("")
    $events = @($sseLines | Where-Object { $_ -match '\| event:' })
    $afterVote = @($events | Where-Object { ($_ -split " ")[0] -gt $voteAt })
    $sseHeaderOk = [bool]($sseLines | Where-Object { $_ -match 'text/event-stream' })
    Add-Check "06c" "Voto por /api/votes" (($rVote.Status -eq 201) -or ($rVote.Status -eq 409)) ("HTTP " + $rVote.Status + $(if ($rVote.Status -eq 409) { " (ya había votado: repetición)" } else { "" }))
    if ($rVote.Status -eq 201) {
        Add-Check "06d" "SSE en vivo por el proxy" ($sseHeaderOk -and $afterVote.Count -ge 1) ("" + $events.Count + " eventos, " + $afterVote.Count + " después del voto")
    } else {
        Add-Check "06d" "SSE en vivo por el proxy" ($sseHeaderOk -and $events.Count -ge 1) ("" + $events.Count + " eventos (sin voto nuevo; usen otro -Documento para ver la actualización)")
    }
    $rRes = Invoke-Http -Url ($Api + "/results/" + $election.id)
    $lines += @("5. Resultados por GET /api/results/" + $election.id + ": HTTP " + $rRes.Status, ("   " + $rRes.Body.Trim()), "")
} else {
    Add-Check "06c" "Voto por /api/votes" $false "no se pudo probar (falta token o elección abierta)"
}
Save-Evidence "06_flujo_completo.txt" $lines

# ---------------------------------------------------------------------------
# 07 · Límite de login por IP (S4 en el proxy) y bloqueo de cuenta (S4 en auth)
# ---------------------------------------------------------------------------
$badBody = Write-BodyFile "login-bad.json" ('{"document":"' + $DocumentoBloqueo + '","password":"clave-incorrecta"}')
$lines = @("8 intentos seguidos de POST /api/auth/login con el documento $DocumentoBloqueo y una clave incorrecta:", "")
$codes = @()
$r429 = $null
for ($i = 1; $i -le 8; $i++) {
    $r = Invoke-Http -Method POST -Url ($Api + "/auth/login") -Headers @("Content-Type: application/json") -BodyFile $badBody
    $codes += $r.Status
    if ($r.Status -eq 429 -and -not $r429) { $r429 = $r }
    $lines += ("  intento {0}: HTTP {1}  {2}" -f $i, $r.Status, $r.Body.Trim())
}
if ($r429) { $lines += @("", "Primera respuesta 429 completa:") + (Format-Response $r429) }
Save-Evidence "07_login_limite.txt" $lines
$codeText = ($codes | ForEach-Object { "$_" }) -join " "
Add-Check "07a" "429 del proxy al pasar el límite" (($null -ne $r429) -and ($r429.Body -match '"detail"') -and -not (Test-ProxyHeaders $r429)) $codeText
Add-Check "07b" "423 del Auth Service (bloqueo S4)" $(if ($codes -contains 423) { $true } else { $null }) $(if ($codes -contains 423) { "cuenta $DocumentoBloqueo bloqueada" } else { "sin 423: el bloqueo de S4 aún no está en esta rama" })

# ---------------------------------------------------------------------------
# 08 · Caída del gateway: 502 en JSON y recuperación sin tocar el proxy
# ---------------------------------------------------------------------------
if (-not $OmitirCaida) {
    $lines = @("> docker compose stop agr-gateway-or")
    $lines += (Invoke-Native "docker" @("compose", "stop", "agr-gateway-or")).Lines
    Start-Sleep -Seconds 2
    $rDown = Invoke-Http -Url ($Api + "/health")
    $rWebDown = Invoke-Http -Url ($Base + "/")
    $lines += @("", "Con el gateway detenido:") + (Format-Response $rDown)
    $lines += @("La web sigue respondiendo (muestra su propio aviso de error):", ("HTTP " + $rWebDown.Status), "")
    $lines += @("> docker compose start agr-gateway-or")
    $lines += (Invoke-Native "docker" @("compose", "start", "agr-gateway-or")).Lines
    $sw = [Diagnostics.Stopwatch]::StartNew()
    $rUp = $null
    while ($sw.Elapsed.TotalSeconds -lt 90) {
        $rUp = Invoke-Http -Url ($Api + "/health") -MaxTime 5
        if ($rUp.Status -eq 200) { break }
        Start-Sleep -Seconds 2
    }
    $lines += @("", ("El proxy vuelve a llegar al gateway a los {0:N0} s, sin reiniciarse (resolver de Docker):" -f $sw.Elapsed.TotalSeconds)) + (Format-Response $rUp)
    Save-Evidence "08_caida_gateway.txt" $lines
    Add-Check "08a" "Gateway caído -> 502 JSON" (($rDown.Status -eq 502) -and ($rDown.Body -match '"detail"')) ("HTTP " + $rDown.Status)
    Add-Check "08b" "Recupera sin reiniciar el proxy" ($rUp.Status -eq 200) ("{0:N0} s" -f $sw.Elapsed.TotalSeconds)
}

# ---------------------------------------------------------------------------
# 09 · Redes
# ---------------------------------------------------------------------------
$lines = @()
foreach ($n in "agr-public-net", "agr-edge-net") {
    $ni = Invoke-Native "docker" @("network", "inspect", $n, "--format", "{{.Name}}  internal={{.Internal}}  contenedores={{range .Containers}}{{.Name}} {{end}}")
    $lines += @("> docker network inspect $n") + $ni.Lines + @("")
}
Save-Evidence "09_redes.txt" $lines
$edgeInternal = ((Invoke-Native "docker" @("network", "inspect", "agr-edge-net", "--format", "{{.Internal}}")).Lines | Select-Object -First 1)
$publicInfo = (Invoke-Native "docker" @("network", "inspect", "agr-public-net", "--format", "{{.Internal}}|{{range .Containers}}{{.Name}} {{end}}")).Lines | Select-Object -First 1
$publicParts = "$publicInfo" -split "\|", 2
$publicMembers = @()
if ($publicParts.Count -gt 1) { $publicMembers = @($publicParts[1].Trim() -split "\s+" | Where-Object { $_ }) }
Add-Check "09a" "agr-edge-net es internal" ($edgeInternal -eq "true") ("internal=" + $edgeInternal)
Add-Check "09b" "En agr-public-net solo está el proxy" (($publicParts[0] -eq "false") -and ($publicMembers.Count -eq 1) -and ($publicMembers[0] -eq "agr-proxy-rp")) (($publicMembers -join ", "))

# ---------------------------------------------------------------------------
# 10 · Configuración efectiva y log del proxy
# ---------------------------------------------------------------------------
$cfg = Invoke-Native "docker" @("compose", "exec", "-T", "agr-proxy-rp", "nginx", "-T")
$log = Invoke-Native "docker" @("compose", "logs", "--no-color", "--tail", "60", "agr-proxy-rp")
Save-Evidence "10_proxy_config_y_log.txt" (@("> docker compose exec agr-proxy-rp nginx -T") + $cfg.Lines + @("", "> docker compose logs --tail 60 agr-proxy-rp") + $log.Lines)
Add-Check "10" "Configuración de Nginx válida" ($cfg.Code -eq 0) ("nginx -T código " + $cfg.Code)

# ---------------------------------------------------------------------------
# 00 · Resumen
# ---------------------------------------------------------------------------
$fails = @($Checks | Where-Object { $_.Estado -eq "FALLA" }).Count
$table = $Checks | Format-Table Id, Estado, Prueba, Detalle -AutoSize -Wrap | Out-String -Width 220
Save-Evidence "00_resumen.txt" (@("Resultado: " + $(if ($fails -eq 0) { "todas las pruebas pasan" } else { "$fails prueba(s) fallan" }), "") + ($table -split "`r?`n"))
Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
Pop-Location

Write-Host ""
if ($fails -eq 0) { Write-Host "Todo OK. Evidencia en p3/s2/despues/" -ForegroundColor Green }
else { Write-Host "$fails prueba(s) fallan. Revisen p3/s2/despues/00_resumen.txt" -ForegroundColor Red }
if ($codes -contains 423) {
    Write-Host ""
    Write-Host "La cuenta $DocumentoBloqueo quedó bloqueada 15 minutos. Para desbloquearla ya:" -ForegroundColor Yellow
    Write-Host ("  docker compose exec -T agr-auth-db psql -U postgres -d agr_auth_db -c ""UPDATE voters SET failed_attempts = 0, locked_until = NULL WHERE document = '" + $DocumentoBloqueo + "';""")
}
Write-Host ""
Write-Host "Capturas que faltan (guárdenlas en p3/s2/capturas/, ver p3/s2/README.md)." -ForegroundColor Cyan
if ($fails -gt 0) { exit 1 }
