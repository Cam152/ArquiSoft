<#
.SYNOPSIS
  S5 - Auditoria a prueba de alteraciones. Altera agr-audit-db a mano y muestra si AGORA lo detecta.

.DESCRIPTION
  Con el sistema arriba, desde la raiz del repositorio:
      docker compose up -d --build --wait
      powershell -ExecutionPolicy Bypass -File .\p3\s5\verificar-auditoria.ps1            # rama p3 (despues)
      powershell -ExecutionPolicy Bypass -File .\p3\s5\verificar-auditoria.ps1 -Fase antes # Prototipo 2 (antes)

  Que hace:
      01  publica 5 eventos VoteRegistered de prueba SOLO en la cola audit.vote-registered
          (exchange por defecto de RabbitMQ: no llegan a agr-results-lo, no cambian resultados)
      02  verifica la cadena con la CLI (agora audit verify): debe estar intacta
      03  ataque A - cambia election_id de un evento en agr-audit-db           -> esperado: altered
      04  ataque B - cambia election_id y rehace el hash con SHA-256 sin llave -> esperado: altered
      05  ataque C - borra un evento de en medio                               -> esperado: missing
      06  ataque D - inserta un evento falso directo en la base                -> esperado: unchained
      07  ataque E - borra el ultimo evento                                    -> esperado: truncated
      Despues de cada ataque se restaura el documento original y se comprueba que la cadena vuelve a estar
      intacta, asi el script se puede repetir.

  Con -Fase antes (rama main / Prototipo 2) solo hace el ataque A y muestra que "agora audit list" lo
  presenta como un evento normal y que no existe ninguna verificacion (404).

  Evidencias en p3/s5/<fase>/ (00_resumen.txt + un archivo por prueba).
#>
param(
    [ValidateSet("antes", "despues")]
    [string]$Fase = "despues",
    [int]$Eventos = 5,
    [long]$Eleccion = 1,
    [string]$RabbitUrl = "http://127.0.0.1:15672",
    [string]$RabbitUser = "agora",
    [string]$RabbitPass = "agora",
    [string]$AuditDbUser = "audit",
    [string]$AuditDbPass = "audit",
    [string]$AuditDb = "audit",
    # Carpeta del docker-compose.yml del sistema que esta corriendo (para la CLI). Por defecto, este repositorio.
    [string]$ComposeDir = "",
    # Solo fase antes: en el Prototipo 2 el gateway se publicaba directo en el 8080.
    [string]$GatewayUrl = "http://localhost:8080",
    [string]$AdminKey = "dev-admin-key"
)

$ErrorActionPreference = "Continue"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
if (-not $ComposeDir) { $ComposeDir = $RepoRoot }
$ComposeDir = (Resolve-Path $ComposeDir).Path
$OutDir = Join-Path $PSScriptRoot $Fase
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Utf8 = New-Object System.Text.UTF8Encoding($false)
$Tmp = Join-Path ([IO.Path]::GetTempPath()) ("agr-s5-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
$Checks = New-Object System.Collections.Generic.List[object]
$Run = [guid]::NewGuid().ToString("N").Substring(0, 8)
Push-Location $ComposeDir   # "docker compose run agr-admin-fr" usa el compose del sistema que corre

# ---------------------------------------------------------------------------
# Utilidades
# ---------------------------------------------------------------------------
function Save-Evidence([string]$Name, [object[]]$Lines) {
    $header = @(
        ("AGORA - Prototipo 3 - S5 Auditoria a prueba de alteraciones - evidencia " + $Fase.ToUpper()),
        ("Fecha: " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz") + "   Rama: " + (git rev-parse --abbrev-ref HEAD 2>$null)),
        ("=" * 78), ""
    )
    $text = (($header + $Lines) | ForEach-Object { "$_" }) -join "`r`n"
    [IO.File]::WriteAllText((Join-Path $OutDir $Name), $text + "`r`n", $Utf8)
    Write-Host ("  guardado  p3/s5/" + $Fase + "/" + $Name) -ForegroundColor DarkGray
}

function Add-Check([string]$Id, [string]$Name, $Ok, [string]$Detail) {
    $estado = "INFO"
    if ($Ok -eq $true) { $estado = "OK" } elseif ($Ok -eq $false) { $estado = "FALLA" }
    $Checks.Add([pscustomobject]@{ Id = $Id; Prueba = $Name; Estado = $estado; Detalle = $Detail })
    $color = @{ "OK" = "Green"; "FALLA" = "Red"; "INFO" = "Yellow" }[$estado]
    Write-Host ("  [{0,-5}] {1} {2}  {3}" -f $estado, $Id, $Name, $Detail) -ForegroundColor $color
}

# Ejecuta un script de mongosh dentro de agr-audit-db (como haria quien entra a la base).
function Invoke-Mongo([string]$Js) {
    $file = Join-Path $Tmp ("s5-" + [guid]::NewGuid().ToString("N") + ".js")
    [IO.File]::WriteAllText($file, $Js, $Utf8)
    docker cp $file "agr-audit-db:/tmp/s5.js" 2>&1 | Out-Null
    $lines = & docker exec agr-audit-db mongosh --quiet -u $AuditDbUser -p $AuditDbPass `
        --authenticationDatabase admin $AuditDb --file /tmp/s5.js 2>&1 | ForEach-Object { "$_" }
    return @($lines)
}

# Ultima linea JSON que imprime un script de mongosh.
function Get-MongoJson([string]$Js) {
    $lines = Invoke-Mongo $Js
    $json = $lines | Where-Object { $_.TrimStart().StartsWith("{") -or $_.TrimStart().StartsWith("[") } | Select-Object -Last 1
    if (-not $json) { return $null }
    return $json | ConvertFrom-Json
}

# CLI de administracion (agr-admin-fr) por la red interna, como en la operacion normal.
function Invoke-Cli([string[]]$CliArgs) {
    $out = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".out")
    $err = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".err")
    $all = @("compose", "--profile", "tools", "run", "--rm", "-T", "agr-admin-fr") + $CliArgs
    $p = Start-Process -FilePath "docker" -ArgumentList $all -NoNewWindow -Wait -PassThru `
        -RedirectStandardOutput $out -RedirectStandardError $err
    $stdout = ""; $stderr = ""
    if (Test-Path $out) { $stdout = [IO.File]::ReadAllText($out, $Utf8) }
    if (Test-Path $err) { $stderr = [IO.File]::ReadAllText($err, $Utf8) }
    # Solo las lineas de la CLI (sin el ruido de docker compose al crear el contenedor).
    $stderr = (($stderr -split "`r?`n") | Where-Object { $_ -match "^Error:" }) -join "`n"
    return [pscustomobject]@{ Code = $p.ExitCode; Out = $stdout.Trim(); Err = $stderr.Trim(); Cmd = ("agora " + ($CliArgs -join " ")) }
}

function Format-Cli($R) {
    $lines = @(("> " + $R.Cmd), ("resultado: " + $R.Code))
    if ($R.Out) { $lines += ($R.Out -split "`r?`n") }
    if ($R.Err) { $lines += ($R.Err -split "`r?`n") }
    return $lines + @("")
}

# Fase antes: el administrador del Prototipo 2 consulta el gateway publicado en el 8080.
function Invoke-Gateway([string]$Path) {
    $out = Join-Path $Tmp ([guid]::NewGuid().ToString("N") + ".b")
    $code = & curl.exe -sS -o $out -w "%{http_code}" -H ("X-API-Key: " + $AdminKey) ($GatewayUrl.TrimEnd("/") + $Path) 2>&1
    $body = ""
    if (Test-Path $out) { $body = [IO.File]::ReadAllText($out, $Utf8) }
    $pretty = $body
    $pretty = $pretty -replace "},{", "},`n{"
    return [pscustomobject]@{ Code = "HTTP $code"; Out = $pretty.Trim(); Err = ""; Cmd = ("curl " + $GatewayUrl.TrimEnd("/") + $Path) }
}

function Invoke-List {
    if ($Fase -eq "antes") { return Invoke-Gateway "/admin/audit?election_id=999" }
    return Invoke-Cli @("audit", "list", "--election", "999")
}

# Verificacion: devuelve el informe (objeto) y la salida legible de la CLI.
function Invoke-Verify {
    if ($Fase -eq "antes") {
        $g = Invoke-Gateway "/admin/audit/verify"
        return [pscustomobject]@{ Report = $null; Text = $g; Json = [pscustomobject]@{ Out = "(no aplica)" } }
    }
    $json = Invoke-Cli @("--json", "audit", "verify")
    $text = Invoke-Cli @("audit", "verify")
    $report = $null
    if ($json.Out.StartsWith("{")) { try { $report = $json.Out | ConvertFrom-Json } catch { } }
    return [pscustomobject]@{ Report = $report; Text = $text; Json = $json }
}

function Find-Problem($Report, [string]$Kind, $Seq) {
    if (-not $Report) { return $null }
    return $Report.problems | Where-Object { $_.kind -eq $Kind -and ($null -eq $Seq -or $_.seq -eq $Seq) } | Select-Object -First 1
}

# Publica un VoteRegistered directo en la cola de auditoria (exchange por defecto).
function Publish-AuditEvent([string]$EventId) {
    $occurred = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    $payload = '{"event_id":"' + $EventId + '","type":"VoteRegistered","version":1,"election_id":' + $Eleccion + ',"occurred_at":"' + $occurred + '"}'
    $body = @{
        properties       = @{ content_type = "application/json"; delivery_mode = 2 }
        routing_key      = "audit.vote-registered"
        payload          = $payload
        payload_encoding = "string"
    } | ConvertTo-Json -Compress
    $file = Join-Path $Tmp ($EventId + ".json")
    [IO.File]::WriteAllText($file, $body, $Utf8)
    $resp = & curl.exe -sS -u ("{0}:{1}" -f $RabbitUser, $RabbitPass) -H "Content-Type: application/json" `
        -X POST "$RabbitUrl/api/exchanges/%2F/amq.default/publish" --data-binary "@$file" 2>&1
    return ("$resp" -match '"routed"\s*:\s*true')
}

# ---------------------------------------------------------------------------
Write-Host ""
Write-Host "AGORA - S5 Auditoria a prueba de alteraciones (fase: $Fase)" -ForegroundColor Cyan
Write-Host ""

$running = docker ps --format "{{.Names}}" 2>$null
foreach ($c in @("agr-audit-db", "agr-audit-lo", "agr-broker-as")) {
    if ($running -notcontains $c) {
        Write-Host "Falta el contenedor $c. Levante el sistema con: docker compose up -d --build --wait" -ForegroundColor Red
        Pop-Location; exit 1
    }
}

# La CLI se construye desde este checkout (si no, podria quedar la imagen de otra version).
if ($Fase -eq "despues") {
    Write-Host "  construyendo la CLI (agr-admin-fr)..." -ForegroundColor DarkGray
    docker compose --profile tools build -q agr-admin-fr 2>&1 | Out-Null
}

# 01 - Eventos de prueba -----------------------------------------------------
$ids = @()
$routed = 0
for ($i = 1; $i -le $Eventos; $i++) {
    $id = "s5-$Run-$i"
    $ids += $id
    if (Publish-AuditEvent $id) { $routed++ }
}
# Espera a que agr-audit-lo los guarde.
$stored = 0
for ($t = 0; $t -lt 30; $t++) {
    $r = Get-MongoJson ('print(JSON.stringify({n: db.events.countDocuments({event_id: {$regex: "^s5-' + $Run + '-"}})}))')
    if ($r) { $stored = [int]$r.n }
    if ($stored -ge $Eventos) { break }
    Start-Sleep -Milliseconds 500
}
$docs = Get-MongoJson ('print(EJSON.stringify(db.events.find({event_id: {$regex: "^s5-' + $Run + '-"}}, {_id: 0}).sort({seq: 1, event_id: 1}).toArray()))')
$docLines = @("Eventos publicados en audit.vote-registered (exchange por defecto, no pasan por agora.votes): $routed/$Eventos",
    "Eventos guardados en agr-audit-db: $stored", "")
if ($docs) { $docLines += ($docs | ForEach-Object { $_ | ConvertTo-Json -Compress }) }
Save-Evidence "01_eventos_de_prueba.txt" $docLines
Add-Check "01" "eventos de prueba guardados" ($stored -ge $Eventos) "$stored de $Eventos"
if ($stored -lt $Eventos) {
    Write-Host "agr-audit-lo no guardo los eventos de prueba; revise: docker compose logs agr-audit-lo" -ForegroundColor Red
    Pop-Location; exit 1
}

# 02 - Linea base ------------------------------------------------------------
$v = Invoke-Verify
$base = @("Estado de la cadena antes de cualquier ataque.", "") + (Format-Cli $v.Text) + @("JSON:", $v.Json.Out)
Save-Evidence "02_cadena_intacta.txt" $base
if ($Fase -eq "despues") {
    $ok = $v.Report -and $v.Report.valid -eq $true
    Add-Check "02" "cadena intacta antes de atacar" $ok ("valid=" + $(if ($v.Report) { $v.Report.valid } else { "?" }) + ", eventos=" + $(if ($v.Report) { $v.Report.total_events } else { "?" }))
    if (-not $ok) {
        Write-Host "La cadena ya estaba alterada antes de empezar (quiza por una ejecucion interrumpida)." -ForegroundColor Red
        Write-Host "Para empezar de cero: docker compose down; docker volume rm <proyecto>_audit-db-data; docker compose up -d --build --wait" -ForegroundColor Red
        Pop-Location; exit 1
    }
} else {
    Add-Check "02" "existe verificacion de la cadena" $null ("GET /admin/audit/verify -> " + $v.Text.Code)
}

$target = $ids[2]   # un evento de en medio
$tail = $ids[$ids.Count - 1]

# Respaldo de un documento (en una coleccion aparte) y restauracion.
function Backup-Doc([string]$EventId) {
    Invoke-Mongo ('db.s5_backup.deleteMany({_id: "' + $EventId + '"}); db.s5_backup.insertOne(db.events.findOne({_id: "' + $EventId + '"}));') | Out-Null
}
function Restore-Doc([string]$EventId) {
    Invoke-Mongo ('const d = db.s5_backup.findOne({_id: "' + $EventId + '"}); db.events.replaceOne({_id: d._id}, d, {upsert: true}); db.s5_backup.deleteOne({_id: d._id});') | Out-Null
}
function Show-Doc([string]$EventId) {
    return @(Invoke-Mongo ('print(EJSON.stringify(db.events.findOne({_id: "' + $EventId + '"})))'))
}

# Ejecuta un ataque, verifica, restaura y vuelve a verificar.
function Invoke-Attack {
    param([string]$Id, [string]$File, [string]$Title, [string]$EventId, [string]$Js, [string]$ExpectedKind, [string]$Explain)
    Backup-Doc $EventId
    $before = Show-Doc $EventId
    $mongoOut = Invoke-Mongo $Js
    $after = Show-Doc $EventId
    $list = Invoke-List
    $v = Invoke-Verify
    Restore-Doc $EventId
    $v2 = Invoke-Verify

    $seq = $null
    $orig = $before | Where-Object { $_.StartsWith("{") } | Select-Object -First 1
    if ($orig) { try { $seq = ($orig | ConvertFrom-Json).seq } catch { } }

    $lines = @($Title, $Explain, "",
        "--- Documento original en agr-audit-db", $before, "",
        "--- Ataque (mongosh dentro de agr-audit-db, con el usuario de la base)", $Js, $mongoOut, "",
        "--- Documento despues del ataque", $after, "",
        "--- Lo que ve el administrador al consultar la auditoria de la eleccion 999") + (Format-Cli $list) + @(
        "--- Verificacion despues del ataque") + (Format-Cli $v.Text) + @("JSON:", $v.Json.Out, "",
        "--- Se restaura el documento original y se verifica otra vez") + (Format-Cli $v2.Text)
    Save-Evidence $File $lines

    if ($Fase -eq "antes") {
        Add-Check $Id $Title $null ("GET /admin/audit lo lista sin aviso (" + $list.Code + "); verificacion: " + $v.Text.Code)
        return
    }
    $seqFilter = $seq
    if ($ExpectedKind -eq "unchained") { $seqFilter = $null }
    $p = Find-Problem $v.Report $ExpectedKind $seqFilter
    $detected = ($null -ne $p) -and $v.Report.valid -eq $false -and $v.Text.Code -eq 1
    $restored = $v2.Report -and $v2.Report.valid -eq $true
    $detail = "esperado $ExpectedKind"
    if ($p) { $detail = "senalado: " + $p.kind + " seq " + $p.seq + " (" + $p.detail + ")" }
    Add-Check $Id $Title $detected $detail
    Add-Check ($Id + "b") "cadena intacta al restaurar" $restored ""
}

# 03 - Ataque A: cambiar un campo --------------------------------------------
Invoke-Attack -Id "03" -File "03_ataque_modificar.txt" -Title "Ataque A: modificar un evento" -EventId $target `
    -ExpectedKind "altered" -Explain "Se cambia election_id del evento en la base, sin tocar seq ni hash." `
    -Js ('printjson(db.events.updateOne({_id: "' + $target + '"}, {$set: {election_id: 999}}));')

if ($Fase -eq "despues") {
    # 04 - Ataque B: modificar y rehacer el hash sin la llave -------------------
    $jsB = @'
const crypto = require("crypto");
const d = db.events.findOne({_id: "__ID__"});
d.election_id = 999;
const iso = (x) => x.toISOString();
const canon = JSON.stringify(["agora-audit-v1", Number(d.seq), d.event_id, d.type, Number(d.election_id), iso(d.occurred_at), iso(d.recorded_at), d.prev_hash]);
const forged = crypto.createHash("sha256").update(canon).digest("hex");
printjson(db.events.updateOne({_id: d._id}, {$set: {election_id: 999, hash: forged}}));
'@ -replace "__ID__", $target
    Invoke-Attack -Id "04" -File "04_ataque_rehacer_hash.txt" -Title "Ataque B: modificar y recalcular el hash" -EventId $target `
        -ExpectedKind "altered" -Explain "El atacante conoce el formato y recalcula hash con SHA-256, pero no tiene AUDIT_CHAIN_KEY (HMAC)." `
        -Js $jsB

    # 05 - Ataque C: borrar uno de en medio --------------------------------------
    Invoke-Attack -Id "05" -File "05_ataque_borrar.txt" -Title "Ataque C: borrar un evento" -EventId $target `
        -ExpectedKind "missing" -Explain "Se borra un evento de en medio de la cadena." `
        -Js ('printjson(db.events.deleteOne({_id: "' + $target + '"}));')

    # 06 - Ataque D: insertar uno falso ------------------------------------------
    $fake = "s5-$Run-falso"
    Invoke-Mongo ('db.s5_backup.deleteMany({_id: "' + $fake + '"});') | Out-Null
    $jsD = 'printjson(db.events.insertOne({_id: "' + $fake + '", event_id: "' + $fake + '", type: "VoteRegistered", election_id: ' + $Eleccion + ', occurred_at: new Date(), recorded_at: new Date()}));'
    Invoke-Mongo ('db.s5_backup.deleteMany({_id: "' + $fake + '"});') | Out-Null
    # Para este ataque "restaurar" es borrar el falso: el respaldo queda vacio y se borra a mano.
    $before = @("(no existe: se va a insertar)")
    $mongoOut = Invoke-Mongo $jsD
    $after = Show-Doc $fake
    $list = Invoke-List
    $v = Invoke-Verify
    Invoke-Mongo ('db.events.deleteOne({_id: "' + $fake + '"});') | Out-Null
    $v2 = Invoke-Verify
    Save-Evidence "06_ataque_insertar.txt" (@("Ataque D: insertar un evento falso", "Se agrega un VoteRegistered directo en la base, sin pasar por el Audit Service.", "",
        "--- Ataque (mongosh dentro de agr-audit-db)", $jsD, $mongoOut, "", "--- Documento insertado", $after, "",
        "--- Lo que ve el administrador al consultar la auditoria de la eleccion 999") + (Format-Cli $list) + @("--- Verificacion despues del ataque") +
        (Format-Cli $v.Text) + @("JSON:", $v.Json.Out, "", "--- Se borra el evento falso y se verifica otra vez") + (Format-Cli $v2.Text))
    $p = Find-Problem $v.Report "unchained" $null
    $detail = "esperado unchained"
    if ($p) { $detail = "senalado: " + $p.kind + " " + $p.event_id }
    Add-Check "06" "Ataque D: insertar un evento falso" (($null -ne $p) -and $v.Text.Code -eq 1) $detail
    Add-Check "06b" "cadena intacta al restaurar" ($v2.Report -and $v2.Report.valid -eq $true) ""

    # 07 - Ataque E: borrar el ultimo --------------------------------------------
    $last = Get-MongoJson 'print(EJSON.stringify(db.events.find({seq: {$exists: true}}).sort({seq: -1}).limit(1).toArray()[0]))'
    $lastId = $tail
    if ($last) { $lastId = $last._id }
    Invoke-Attack -Id "07" -File "07_ataque_truncar.txt" -Title "Ataque E: borrar el ultimo evento" -EventId $lastId `
        -ExpectedKind "truncated" -Explain "Se borra el evento mas reciente: no queda ningun hueco en la cadena, pero el servicio conoce la cabeza." `
        -Js ('printjson(db.events.deleteOne({_id: "' + $lastId + '"}));')
}

# En el Prototipo 2 no hay cadena: se borran los eventos de prueba para no dejar basura.
if ($Fase -eq "antes") {
    Invoke-Mongo 'printjson(db.events.deleteMany({event_id: {$regex: "^s5-"}})); db.s5_backup.drop();' | Out-Null
}

# ---------------------------------------------------------------------------
$sum = @(("Prueba".PadRight(48) + "Estado".PadRight(7) + "Detalle"), ("-" * 78))
foreach ($c in $Checks) { $sum += ($c.Id + " " + $c.Prueba).PadRight(48) + $c.Estado.PadRight(7) + $c.Detalle }
$fails = @($Checks | Where-Object { $_.Estado -eq "FALLA" }).Count
$sum += @("", "Fallas: $fails")
Save-Evidence "00_resumen.txt" $sum

Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
Pop-Location
Write-Host ""
if ($fails -gt 0) { Write-Host "Hay $fails falla(s). Ver p3/s5/$Fase/00_resumen.txt" -ForegroundColor Red; exit 1 }
Write-Host "Listo. Evidencias en p3/s5/$Fase/" -ForegroundColor Green
