<#
  B5 · agr-web-fr solo alcanza a agr-gateway-or (versión para PowerShell)
  Laboratorio 5 (Network Segmentation) · Equipo 1b · Responsable: Cristian

  Hace las mismas comprobaciones que verify-b5-web-fr.sh, desde el espacio de
  red de agr-web-fr:
    1. agr-web-fr está solo en agr-edge-net, publica solo el 3000 y no guarda
       secretos en su entorno;
    2. el flujo permitido F3 (agr-web-fr -> agr-gateway-or:8080) funciona;
    3. ningún *-lo, *-db ni el broker resuelve ni conecta por nombre;
    4. tampoco conectan por IP: el aislamiento no depende solo del DNS;
    5. ningún puerto publicado en el host abre un camino que salte la
       segmentación;
    6. (informativo) si agr-web-fr tiene salida a internet.

  Las pruebas corren en un contenedor busybox que comparte la red de
  agr-web-fr (--network container:agr-web-fr), así que no hace falta instalar
  nada en la imagen del front-end.

  Uso, desde la raíz del repositorio y con el sistema arriba:
    docker compose up --build -d --wait
    Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
    .\lab5-security\scripts\verify-b5-web-fr.ps1 [archivo-de-salida]

  Por defecto guarda una copia en lab5-security\evidence\after\B5_web-fr.txt.
  Código de salida: 0 si todo salió como se esperaba; 1 si hubo fallas;
  2 si no se pudo ejecutar.
  Compatible con Windows PowerShell 5.1 y PowerShell 7.
#>
param([string]$Salida)

$ErrorActionPreference = 'Continue'

$ORIGEN = 'agr-web-fr'
$GATEWAY = 'agr-gateway-or'
$GATEWAY_PUERTO = 8080
$RED_ESPERADA = 'agr-edge-net'
$IMAGEN = 'busybox'
if ($env:BUSYBOX_IMAGE) { $IMAGEN = $env:BUSYBOX_IMAGE }

# Destinos que agr-web-fr no debe alcanzar (design/flujos-permitidos.md, B5).
$DESTINOS = @(
  'agr-auth-lo:8081', 'agr-voter-lo:8001', 'agr-election-lo:8000',
  'agr-vote-lo:8002', 'agr-results-lo:8004', 'agr-audit-lo:8005',
  'agr-auth-db:5432', 'agr-voters-db:5432', 'agr-elections-db:5432',
  'agr-votes-db:27017', 'agr-results-db:6379', 'agr-audit-db:27017',
  'agr-broker-as:5672'
)

# ------------------------------------------------------------------ salida
$dirLab = Split-Path -Parent $PSScriptRoot
if (-not $Salida) {
  if (Test-Path (Join-Path $dirLab 'evidence')) {
    $Salida = Join-Path (Join-Path (Join-Path $dirLab 'evidence') 'after') 'B5_web-fr.txt'
  } else {
    $Salida = 'B5_web-fr.txt'
  }
}
$Salida = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Salida)
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Salida) | Out-Null
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($Salida, '', $utf8)

function Escribir([string]$texto) {
  [System.IO.File]::AppendAllText($Salida, $texto + [Environment]::NewLine, $utf8)
}

$script:TOTAL = 0
$script:FALLAS = 0
$script:AVISOS = 0

function Linea([string]$etiqueta, [string]$color, [string]$texto) {
  Write-Host '  ' -NoNewline
  Write-Host ('{0,-9}' -f $etiqueta) -ForegroundColor $color -NoNewline
  Write-Host $texto
  Escribir ('  {0,-9}{1}' -f $etiqueta, $texto)
}
function Ok([string]$t)       { $script:TOTAL++; Linea 'OK' 'Green' $t }
function Falla([string]$t)    { $script:TOTAL++; $script:FALLAS++; Linea 'FALLA' 'Red' $t }
function Atencion([string]$t) { $script:AVISOS++; Linea 'ATENCIÓN' 'Yellow' $t }
function Info([string]$t)     { Linea 'INFO' 'DarkGray' $t }
function Texto([string]$t)    { Write-Host $t; Escribir $t }
function Titulo([string]$t)   { Write-Host ''; Write-Host $t -ForegroundColor White; Escribir ''; Escribir $t }

# Ejecuta docker con los argumentos dados y devuelve su salida (stdout y stderr)
# como texto. Deja el código de salida en $script:CODIGO.
function Invoke-Docker {
  $salidaDocker = & docker @args 2>&1 | ForEach-Object { "$_" }
  $script:CODIGO = $LASTEXITCODE
  return (@($salidaDocker) -join "`n")
}

# Resolver NOMBRE: pregunta al DNS de Docker desde agr-web-fr. Deja la primera
# IPv4 en $script:RES_IP (vacía si no resuelve) y la causa en $script:DNS_MSG.
function Resolver([string]$nombre) {
  $t = Invoke-Docker exec $SONDA nslookup -type=a $nombre
  $script:RES_IP = ''
  $visto = $false
  foreach ($l in ($t -split "`n")) {
    if ($l -match '^Name:') { $visto = $true; continue }
    if ($visto -and $l -match '^Address:?\s+(\S+)') { $script:RES_IP = $Matches[1]; break }
  }
  if ($t -match 'NXDOMAIN') { $script:DNS_MSG = 'NXDOMAIN' } else { $script:DNS_MSG = 'sin respuesta' }
}

# Tcp DESTINO PUERTO [SEGUNDOS]: $true si la conexión se abre. Deja el mensaje de
# nc en $script:TCP_MSG.
function Tcp([string]$destino, [string]$puerto, [int]$seg = 2) {
  $t = Invoke-Docker exec $SONDA nc -zv -w $seg $destino $puerto
  $script:TCP_MSG = [string](($t -split "`n") | Where-Object { $_ } | Select-Object -Last 1)
  return ($script:CODIGO -eq 0)
}

# Resume el mensaje de nc: abierto, bad address, timeout, sin ruta o cerrado.
function Motivo([string]$m) {
  if ($m -match 'open') { return 'abierto' }
  if ($m -match 'bad address') { return 'bad address' }
  if ($m -match 'timed out') { return 'timeout' }
  if ($m -match 'unreachable|No route') { return 'sin ruta' }
  return 'cerrado'
}

# ---------------------------------------------------------- precondiciones
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  Write-Host 'No se encontró el comando docker.' -ForegroundColor Red
  exit 2
}
$estado = (Invoke-Docker inspect -f '{{.State.Status}}' $ORIGEN).Trim()
if ($script:CODIGO -ne 0) {
  Write-Host "No existe el contenedor $ORIGEN. Levanten el sistema: docker compose up --build -d --wait" -ForegroundColor Red
  exit 2
}
if ($estado -ne 'running') {
  Write-Host "$ORIGEN está en estado '$estado'. Revisen: docker compose logs $ORIGEN --tail 50" -ForegroundColor Red
  exit 2
}
Invoke-Docker image inspect $IMAGEN | Out-Null
if ($script:CODIGO -ne 0) {
  Invoke-Docker pull -q $IMAGEN | Out-Null
  if ($script:CODIGO -ne 0) { Write-Host "No se pudo descargar la imagen $IMAGEN." -ForegroundColor Red; exit 2 }
}
$SONDA = "b5-sonda-$PID"
Invoke-Docker run -d --rm --name $SONDA --network "container:$ORIGEN" $IMAGEN sleep 900 | Out-Null
if ($script:CODIGO -ne 0) {
  Write-Host "No se pudo crear el contenedor auxiliar en la red de $ORIGEN." -ForegroundColor Red
  exit 2
}

$codigoSalida = 1
try {
  $rama = '?'; $commit = '?'
  if (Get-Command git -ErrorAction SilentlyContinue) {
    $r = (& git -C $dirLab rev-parse --abbrev-ref HEAD 2>$null); if ($r) { $rama = $r }
    $c = (& git -C $dirLab rev-parse --short HEAD 2>$null); if ($c) { $commit = $c }
  }
  $version = (Invoke-Docker version -f '{{.Server.Version}}').Trim()

  Write-Host "B5 · $ORIGEN solo alcanza a $GATEWAY" -ForegroundColor White
  Escribir "B5 · $ORIGEN solo alcanza a $GATEWAY"
  Texto ("{0} · rama {1} ({2}) · Docker {3}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz'), $rama, $commit, $version)

  # ------------------------------------------------------ 1. configuración
  Titulo "1. Configuración de $ORIGEN"

  $redes = @((Invoke-Docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' $ORIGEN) -split '\s+' | Where-Object { $_ })
  $redesStr = $redes -join ' '
  if ($redesStr -eq $RED_ESPERADA) {
    Ok "redes: $redesStr"
  } else {
    if (-not $redesStr) { $redesStr = 'ninguna' }
    Falla "redes: $redesStr (se esperaba solo $RED_ESPERADA)"
  }

  $puertos = @((Invoke-Docker port $ORIGEN) -split "`n" | Where-Object { $_ -match '->' } | ForEach-Object { ($_ -split ' -> ')[1].Trim() })
  $otros = @($puertos | Where-Object { ($_ -split ':')[-1] -ne '3000' })
  if ($puertos.Count -gt 0 -and $otros.Count -eq 0) {
    Ok "puertos publicados: $($puertos -join ' ')"
  } else {
    $p = $puertos -join ' '; if (-not $p) { $p = 'ninguno' }
    Falla "puertos publicados: $p (se esperaba solo el 3000)"
  }

  foreach ($red in $redes) {
    $interna = (Invoke-Docker network inspect -f '{{.Internal}}' $red).Trim()
    $miembros = @((Invoke-Docker network inspect -f '{{range .Containers}}{{.Name}} {{end}}' $red) -split '\s+' | Where-Object { $_ } | Sort-Object)
    $inesperados = @($miembros | Where-Object { $_ -ne $ORIGEN -and $_ -ne $GATEWAY -and $_ -notlike '*admin-fr*' -and $_ -ne $SONDA })
    if ($inesperados.Count -eq 0) {
      Ok "${red}: internal=$interna; miembros: $($miembros -join ' ')"
    } else {
      Falla "${red}: también están $($inesperados -join ' ')"
    }
  }
  if ($redesStr -eq $RED_ESPERADA) {
    Info "$RED_ESPERADA no puede ser internal: publica 3000 y 8080 en el host"
  }

  $entorno = @((Invoke-Docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' $ORIGEN) -split "`n" | Where-Object { $_ })
  $secretos = @($entorno | ForEach-Object { ($_ -split '=', 2)[0] } | Where-Object { $_ -cmatch 'KEY|SECRET|PASS|TOKEN|JWT|CRED' })
  if ($secretos.Count -eq 0) {
    Ok 'sin secretos en el entorno (ninguna variable *KEY*, *SECRET*, *PASS*, *TOKEN*)'
  } else {
    Falla "variables con posibles secretos: $($secretos -join ' ')"
  }
  $urls = @($entorno | Where-Object { $_ -cmatch '^[A-Z_]*GATEWAY[A-Z_]*=' })
  if ($urls.Count -gt 0) { Info ($urls -join ' ') }

  # ---------------------------------------------------------- 2. permitido
  Titulo "2. Flujo permitido F3: $ORIGEN -> ${GATEWAY}:$GATEWAY_PUERTO"

  Resolver $GATEWAY
  if ($script:RES_IP) { Ok "DNS   $GATEWAY -> $($script:RES_IP)" } else { Falla "DNS   ${GATEWAY}: $($script:DNS_MSG)" }
  if (Tcp $GATEWAY $GATEWAY_PUERTO) {
    Ok "TCP   ${GATEWAY}:$GATEWAY_PUERTO abierto"
  } else {
    Falla "TCP   ${GATEWAY}:${GATEWAY_PUERTO}: $(Motivo $script:TCP_MSG)"
  }
  $cuerpo = Invoke-Docker exec $SONDA wget -q -T 5 -O - "http://${GATEWAY}:$GATEWAY_PUERTO/health"
  if ($script:CODIGO -eq 0) {
    $c = $cuerpo -replace "[`r`n]", ''
    if ($c.Length -gt 60) { $c = $c.Substring(0, 60) }
    Ok "HTTP  GET /health -> $c"
  } else {
    Falla "HTTP  GET /health: $((($cuerpo -split "`n") | Select-Object -Last 1))"
  }

  # ---------------------------------------------- 3. bloqueados por nombre
  Titulo '3. B5 por nombre: ningún *-lo, *-db ni broker debe resolver ni conectar'
  Texto ('           {0,-18} {1,-7} {2,-16} {3}' -f 'destino', 'puerto', 'DNS (nslookup)', 'TCP (nc)')

  foreach ($d in $DESTINOS) {
    $nombre, $puerto = $d -split ':'
    $st = (Invoke-Docker inspect -f '{{.State.Status}}' $nombre).Trim()
    if ($st -ne 'running') {
      Falla ('{0,-18} {1,-7} {2}' -f $nombre, $puerto, 'no está corriendo: la prueba no vale')
      continue
    }
    Resolver $nombre
    $ip = $script:RES_IP
    $dns = $script:DNS_MSG
    if ($ip) { $dns = $ip }
    $tcpAbierto = Tcp $nombre $puerto
    $linea = '{0,-18} {1,-7} {2,-16} {3}' -f $nombre, $puerto, $dns, (Motivo $script:TCP_MSG)
    if (-not $ip -and -not $tcpAbierto) { Ok $linea } else { Falla $linea }
  }

  # ------------------------------------------------- 4. bloqueados por IP
  Titulo '4. B5 por IP: aun conociendo la IP del destino, no hay conexión'

  # Prueba todas las IP de un destino en paralelo, con 1 s de espera cada una.
  $scriptSh = 'p=$1; shift; for ip in $@; do ( nc -z -w 1 $ip $p >/dev/null 2>&1 && echo $ip ) & done; wait'
  foreach ($d in $DESTINOS) {
    $nombre, $puerto = $d -split ':'
    $ips = @((Invoke-Docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' $nombre) -split '\s+' | Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' })
    if ($ips.Count -eq 0) {
      Falla ('{0,-18} {1}' -f $nombre, 'sin IP (¿está corriendo?)')
      continue
    }
    $abiertas = @((Invoke-Docker exec $SONDA sh -c $scriptSh sh $puerto @ips) -split '\s+' | Where-Object { $_ })
    $destinoTxt = (@($ips | ForEach-Object { "${_}:$puerto" })) -join ' '
    if ($abiertas.Count -eq 0) {
      Ok ('{0,-18} {1,-14} {2}' -f $nombre, 'sin conexión', $destinoTxt)
    } else {
      Falla ('{0,-18} {1,-14} {2}' -f $nombre, 'ABIERTO', "$($abiertas -join ' ') (de: $destinoTxt)")
    }
  }

  # ---------------------------------------------- 5. puertos del host
  Titulo '5. Puertos publicados en el host: ninguno debe abrir un camino alterno'

  $extra = New-Object System.Collections.Generic.List[string]
  $consola = New-Object System.Collections.Generic.List[string]
  $publicados = New-Object System.Collections.Generic.List[string]
  $contenedores = @((Invoke-Docker ps --format '{{.Names}}') -split '\s+' | Where-Object { $_ -like 'agr-*' } | Sort-Object)
  foreach ($cont in $contenedores) {
    $lineasPuerto = @((Invoke-Docker port $cont) -split "`n" | Where-Object { $_ -match '->' })
    foreach ($l in $lineasPuerto) {
      $hostPub = ($l -split ' -> ')[1].Trim()
      $hport = ($hostPub -split ':')[-1]
      $hip = $hostPub.Substring(0, $hostPub.LastIndexOf(':'))
      $publicados.Add("$cont $hostPub")
      if (($cont -eq $ORIGEN -and $hport -eq '3000') -or ($cont -eq $GATEWAY -and $hport -eq "$GATEWAY_PUERTO")) { continue }
      if ($cont -eq 'agr-broker-as' -and $hport -eq '15672') {
        if ($hip -ne '127.0.0.1' -and $hip -ne '[::1]') { $consola.Add($hostPub) }
        continue
      }
      $extra.Add("$cont $hostPub")
    }
  }

  $pubTxt = 'ninguno'
  if ($publicados.Count -gt 0) { $pubTxt = $publicados -join '; ' }
  Info "publicados: $pubTxt"
  if ($extra.Count -eq 0 -and $consola.Count -eq 0) {
    Ok 'solo se publican 3000 (web), 8080 (gateway) y 127.0.0.1:15672 (consola)'
  }
  foreach ($h in $consola) { Falla "agr-broker-as publica la consola en ${h}; debe ser solo 127.0.0.1:15672" }
  foreach ($e in $extra) { Falla "puerto publicado de más: $e (revisen ports: y si hay un docker-compose.override.yml)" }

  $hostGw = ''
  foreach ($l in ((Invoke-Docker exec $SONDA ip route) -split "`n")) {
    if ($l -match '^default via (\S+)') { $hostGw = $Matches[1]; break }
  }
  if (-not $hostGw) {
    Info 'agr-web-fr no tiene ruta por defecto; no se prueba el acceso por el host'
  } else {
    if (Tcp $hostGw $GATEWAY_PUERTO 2) {
      Info "${hostGw}:$GATEWAY_PUERTO abierto: es la API pública del gateway, el mismo camino de F2"
    }
    if (Tcp $hostGw 15672 2) {
      Falla "${hostGw}:15672 abierto: la consola de RabbitMQ se alcanza a través del host"
    } else {
      Ok "${hostGw}:15672 $(Motivo $script:TCP_MSG): la consola de RabbitMQ no se alcanza por el host"
    }
    foreach ($e in $extra) {
      $p = ($e -split ':')[-1]
      if (Tcp $hostGw $p 2) { Info "${hostGw}:$p abierto: $(($e -split ' ')[0]) se alcanza a través del host" }
    }
  }

  # Solo en Docker Desktop: host.docker.internal lleva a los puertos del equipo,
  # incluidos los publicados en 127.0.0.1.
  Resolver 'host.docker.internal'
  if ($script:RES_IP) {
    if (Tcp 'host.docker.internal' 15672 2) {
      Atencion 'host.docker.internal:15672 abierto (Docker Desktop): la consola se alcanza desde los contenedores'
    } else {
      Ok "host.docker.internal:15672 $(Motivo $script:TCP_MSG)"
    }
  }

  # ------------------------------------------------------- 6. internet
  Titulo '6. Salida a internet (informativo; B4 solo exige el bloqueo a los *-lo)'

  Invoke-Docker exec $SONDA wget -q -T 4 -O /dev/null http://example.com | Out-Null
  if ($script:CODIGO -eq 0) {
    Atencion "$ORIGEN sí sale a internet: $RED_ESPERADA no es internal (riesgo residual, ver §3.6)"
  } else {
    Info "$ORIGEN no llegó a http://example.com"
  }

  # -------------------------------------------------------- resumen
  Texto ''
  if ($script:FALLAS -eq 0) {
    $res = "Resultado: $($script:TOTAL)/$($script:TOTAL) comprobaciones como se esperaba. B5 se cumple."
    Write-Host $res -ForegroundColor Green
    $codigoSalida = 0
  } else {
    $res = "Resultado: $($script:FALLAS) de $($script:TOTAL) comprobaciones fallaron."
    Write-Host $res -ForegroundColor Red
  }
  Escribir $res
  if ($script:AVISOS -gt 0) { Texto "Avisos para documentar: $($script:AVISOS)." }
  Texto "Copia en: $Salida"
}
finally {
  Invoke-Docker rm -f $SONDA | Out-Null
}
exit $codigoSalida
