param([switch]$PrepareOnly)
$ErrorActionPreference = 'Stop'

function Set-JsonValue($Object, [string]$Name, $Value) {
    $Object | Add-Member -MemberType NoteProperty -Name $Name -Value $Value -Force
}
function Read-SHA256([string]$Path) {
    $stream = [IO.File]::OpenRead($Path)
    $hash = [Security.Cryptography.SHA256]::Create()
    try { return [BitConverter]::ToString($hash.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
    finally { $hash.Dispose(); $stream.Dispose() }
}
function Read-PrivateJson([string]$Path) {
    if ((Get-Item -LiteralPath $Path).Length -gt 4MB) { throw 'Configuracao privada excede 4 MB.' }
    return Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json
}
function Write-PrivateJson([string]$Path, $Object) {
    $temporary = $Path + '.' + [Guid]::NewGuid().ToString('N') + '.tmp'
    try {
        [IO.File]::WriteAllText($temporary, ($Object | ConvertTo-Json -Depth 100), (New-Object Text.UTF8Encoding($false)))
        if (Test-Path -LiteralPath $Path) {
            $backup = $Path + '.before-p16-' + [Guid]::NewGuid().ToString('N') + '.bak'
            [IO.File]::Replace($temporary, $Path, $backup)
        } else { [IO.File]::Move($temporary, $Path) }
    } finally {
        if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary }
    }
}

$package = $PSScriptRoot
$agent = Join-Path $package 'app\hosts-p16\HostAgent.exe'
$server = Join-Path $package 'app\server-dinamico-p16\openomsi.exe'
if (!(Test-Path -LiteralPath (Join-Path $package 'HostAgent.exe')) -or !(Test-Path -LiteralPath $server)) {
    throw 'Extraia este ZIP na pasta da Bridge do SERVIDOR, junto do HostAgent.exe antigo. O servidor app\server-dinamico-p16\openomsi.exe precisa estar instalado.'
}
$manifest = Get-Content -LiteralPath (Join-Path $package 'P16-HOST-REPAIR.json') -Raw | ConvertFrom-Json
foreach ($entry in $manifest.files.PSObject.Properties) {
    $path = Join-Path $package $entry.Name
    if (!(Test-Path -LiteralPath $path) -or (Read-SHA256 $path) -ne $entry.Value) {
        throw ('Arquivo da correcao incompleto ou alterado: ' + $entry.Name)
    }
}
foreach ($dll in @('steam_api64.dll','msvcp140.dll','vcruntime140.dll','vcruntime140_1.dll')) {
    if (!(Test-Path -LiteralPath (Join-Path (Split-Path $server) $dll))) {
        throw ('Falta ' + $dll + ' na pasta do servidor P16. Aplique antes a correcao das DLLs.')
    }
}
$privateDir = Join-Path $env:LOCALAPPDATA 'OpenOmsi-BBS'
$target = Join-Path $privateDir 'HostAgent-P16.local.json'
$source = $target
if (!(Test-Path -LiteralPath $source)) {
    $source = Join-Path $privateDir 'HostAgent.local.json'
    if (!(Test-Path -LiteralPath $source)) { $source = Join-Path $package 'HostAgent.local.json' }
}
if (!(Test-Path -LiteralPath $source)) { throw 'Configuracao anterior do HostAgent nao encontrada. Abra o painel Empresa / servidor para configura-la primeiro.' }
$config = Read-PrivateJson $source
if ($null -eq $config.company -or $null -eq $config.maps -or [string]::IsNullOrWhiteSpace($config.root)) {
    throw 'Configuracao anterior incompleta. Nenhum arquivo original foi alterado.'
}
$selected = [IO.Path]::GetFullPath([string]$config.server)
$known = @('app\server\openomsi.exe','app\server-dinamico-p14\openomsi.exe','app\server-dinamico-p15\openomsi.exe','app\server-dinamico-p16\openomsi.exe')
$managed = $false
foreach ($relative in $known) {
    if ($selected -eq [IO.Path]::GetFullPath((Join-Path $package $relative))) { $managed = $true }
}
if (!$managed) { throw 'O perfil seleciona um servidor personalizado fora desta Bridge. Ele foi preservado. Esta correcao deve ser extraida na instalacao que contem o servidor configurado.' }

if (!$PrepareOnly) {
    # Request an authenticated, local-only shutdown. Never send private keys to the directory.
    foreach ($oldPath in @((Join-Path $privateDir 'HostAgent.local.json'), $target, (Join-Path $package 'HostAgent.local.json'))) {
        if (!(Test-Path -LiteralPath $oldPath)) { continue }
        $old = Read-PrivateJson $oldPath
        $port = [int]$old.control_port
        if ($port -lt 1 -or $port -gt 65535 -or [string]::IsNullOrWhiteSpace($old.control_key)) { continue }
        try {
            Invoke-RestMethod -Uri ('http://127.0.0.1:' + $port + '/stop') -Method Post -Headers @{Authorization=('Bearer ' + $old.control_key)} -TimeoutSec 4 | Out-Null
        } catch { } # An agent which is already offline needs no stop request.
    }
    # Close only host windows/processes from this installation, after graceful shutdown.
    $owned = @((Join-Path $package 'HostAgent.exe'), $agent, (Join-Path $package 'CompanyHost.exe'), (Join-Path $package 'app\hosts-p16\CompanyHost.exe'))
    foreach ($process in @(Get-Process -Name HostAgent,CompanyHost -ErrorAction SilentlyContinue)) {
        try { $path = $process.Path } catch { continue }
        if ($owned -notcontains $path) { continue }
        if (!$process.HasExited) { $null = $process.CloseMainWindow() }
        if (!$process.WaitForExit(10000)) { throw 'O agente antigo ainda esta encerrando. Aguarde e execute Abrir-Servidor-P16.cmd novamente.' }
    }
}
Set-JsonValue $config 'server' $server
Set-JsonValue $config 'use_custom_server' $true
Set-JsonValue $config.company 'protocol' 16
Set-JsonValue $config.company 'openomsi_version' '0.3.47'
New-Item -ItemType Directory -Path $privateDir -Force | Out-Null
Write-PrivateJson $target $config
Write-Host 'Perfil P16 preparado. Mapas, portas, chaves, voz e opcoes foram preservados.'
if (!$PrepareOnly) {
    # An explicit configuration path makes this independent of old desktop shortcuts.
    Start-Process -FilePath $agent -ArgumentList @('--config', ('"' + $target + '"')) -WorkingDirectory $package
    Write-Host 'No painel que abriu, clique em Salvar e iniciar agente. Depois inicie a viagem pelo BCS.'
}
