<#
.SYNOPSIS
  S1 - Secure Channel. Captura el trafico del login en el puerto 80 (HTTP) o 443 (HTTPS) del proxy y lo guarda
  como evidencia (archivo .pcap para Wireshark y texto legible).

.DESCRIPTION
  Que hace: arranca tcpdump dentro de la red del contenedor agr-proxy-rp (imagen nicolaka/netshoot, no hace falta
  instalar Wireshark ni Npcap), envia UN login con curl.exe y guarda lo que paso por el cable.

  ANTES de S1 (rama main, solo HTTP): el documento y la clave se ven en texto plano.
      git switch main
      docker compose up -d --build --wait
      powershell -ExecutionPolicy Bypass -File .\p3\s1\capturar-trafico.ps1 -Fase antes -Puerto 80

  DESPUES de S1 (rama p3-miguel, con certificado generado): el 443 solo muestra bytes cifrados.
      git switch p3-miguel
      bash p3/s1/gen-certs.sh
      docker compose up -d --build --wait
      powershell -ExecutionPolicy Bypass -File .\p3\s1\capturar-trafico.ps1 -Fase despues -Puerto 443

  Opcional, para documentar el caso limite (un cliente que empieza por HTTP):
      powershell -ExecutionPolicy Bypass -File .\p3\s1\capturar-trafico.ps1 -Fase despues -Puerto 80

  Salida en p3/s1/antes/ o p3/s1/despues/:
      captura_puerto<N>.pcap   captura completa (se abre con Wireshark)
      captura_puerto<N>.txt    la misma captura en texto (tcpdump -A)
      resumen_puerto<N>.txt    resultado: si el documento y la clave se ven o no
#>
param(
    [ValidateSet("antes", "despues")][string]$Fase = "despues",
    [ValidateSet(80, 443)][int]$Puerto = 443,
    [string]$Documento = "1001",
    [string]$Clave = "123456",
    [string]$Imagen = "nicolaka/netshoot"
)

$ErrorActionPreference = "Continue"
$OutDir = Join-Path $PSScriptRoot $Fase
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Utf8 = New-Object System.Text.UTF8Encoding($false)
$Pcap = "captura_puerto$Puerto.pcap"
$TxtFile = Join-Path $OutDir "captura_puerto$Puerto.txt"
$ResFile = Join-Path $OutDir "resumen_puerto$Puerto.txt"

function Invoke-Native([string]$Exe, [string[]]$ArgList) {
    $lines = & $Exe @ArgList 2>&1 | ForEach-Object { "$_" }
    return [pscustomobject]@{ Code = $LASTEXITCODE; Lines = @($lines) }
}

$running = ((Invoke-Native "docker" @("inspect", "-f", "{{.State.Running}}", "agr-proxy-rp")).Lines | Select-Object -First 1)
if ($running -ne "true") {
    Write-Host "agr-proxy-rp no esta en ejecucion. Levante el sistema con: docker compose up -d --build --wait" -ForegroundColor Red
    exit 1
}

Write-Host "Captura en el puerto $Puerto (fase: $Fase)..." -ForegroundColor Cyan
$null = Invoke-Native "docker" @("rm", "-f", "agr-s1-sniffer")
$start = Invoke-Native "docker" @("run", "-d", "--name", "agr-s1-sniffer", "--net", "container:agr-proxy-rp",
                                  "-v", "${OutDir}:/out", $Imagen,
                                  "tcpdump", "-i", "any", "-s", "0", "-U", "-w", "/out/$Pcap", "tcp port $Puerto")
if ($start.Code -ne 0) {
    Write-Host ("No se pudo iniciar tcpdump: " + ($start.Lines -join " ")) -ForegroundColor Red
    exit 1
}
Start-Sleep -Seconds 3   # tcpdump necesita un momento para empezar a escuchar

# Un solo login (credenciales correctas de un votante de prueba).
$body = Join-Path ([IO.Path]::GetTempPath()) ("agr-s1-login-" + [guid]::NewGuid().ToString("N") + ".json")
[IO.File]::WriteAllText($body, ('{"document":"' + $Documento + '","password":"' + $Clave + '"}'), $Utf8)
if ($Puerto -eq 443) { $url = "https://localhost/api/auth/login" } else { $url = "http://localhost/api/auth/login" }
$curlArgs = @("-sS", "-k", "-i", "--max-time", "15", "-X", "POST", "-H", "Content-Type: application/json", "--data-binary", "@$body", $url)
$resp = Invoke-Native "curl.exe" $curlArgs
Remove-Item -Force $body -ErrorAction SilentlyContinue
$status = ($resp.Lines | Where-Object { $_ -match '^HTTP/' } | Select-Object -First 1)
Write-Host ("Respuesta del proxy: " + $status)

Start-Sleep -Seconds 2
$null = Invoke-Native "docker" @("stop", "agr-s1-sniffer")
$null = Invoke-Native "docker" @("rm", "-f", "agr-s1-sniffer")

# La captura en texto: tcpdump -A muestra el contenido de cada paquete como ASCII.
$dump = Invoke-Native "docker" @("run", "--rm", "-v", "${OutDir}:/out:ro", $Imagen, "tcpdump", "-r", "/out/$Pcap", "-nn", "-A")
[IO.File]::WriteAllText($TxtFile, (($dump.Lines) -join "`r`n") + "`r`n", $Utf8)
$text = $dump.Lines -join "`n"

$seesDoc = $text -match [regex]::Escape('"document":"' + $Documento + '"')
$seesPwd = $text -match [regex]::Escape('"password":"' + $Clave + '"')
$seesJwt = $text -match 'access_token'
$packets = @($dump.Lines | Where-Object { $_ -match '^\d\d:\d\d:\d\d' }).Count

$res = @()
$res += "AGORA - Prototipo 3 - S1 Secure Channel - captura de trafico (fase: $Fase, puerto: $Puerto)"
$res += "Fecha: " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz")
$res += ("=" * 78)
$res += "Peticion enviada: POST $url  (documento $Documento)"
$res += "Respuesta: $status"
$res += "Paquetes capturados en el puerto ${Puerto}: $packets"
$res += ""
$res += "Documento legible en la captura : " + $(if ($seesDoc) { "SI" } else { "NO" })
$res += "Clave legible en la captura     : " + $(if ($seesPwd) { "SI" } else { "NO" })
$res += "Token JWT legible en la captura : " + $(if ($seesJwt) { "SI" } else { "NO" })
$res += ""
if ($seesPwd) {
    $res += "Lineas de la captura con las credenciales:"
    $res += @($dump.Lines | Where-Object { $_ -match 'document|password|access_token' } | Select-Object -First 6 | ForEach-Object { "  " + $_ })
} else {
    $res += "No aparece ninguna credencial en claro: el contenido viaja cifrado (TLS)."
}
[IO.File]::WriteAllText($ResFile, ($res -join "`r`n") + "`r`n", $Utf8)

Write-Host ""
$res | ForEach-Object { Write-Host $_ }
Write-Host ""
Write-Host "Guardado en p3/s1/$Fase/ : $Pcap, captura_puerto$Puerto.txt y resumen_puerto$Puerto.txt" -ForegroundColor Green
if ($Fase -eq "antes" -and -not $seesPwd) { Write-Host "AVISO: se esperaba ver la clave en texto plano antes de S1. Revise la rama y el puerto." -ForegroundColor Yellow }
if ($Fase -eq "despues" -and $Puerto -eq 443 -and $seesPwd) { Write-Host "FALLA: la clave se ve en claro en el 443." -ForegroundColor Red; exit 1 }
