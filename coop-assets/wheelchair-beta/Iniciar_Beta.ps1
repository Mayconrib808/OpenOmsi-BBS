param([Parameter(ValueFromRemainingArguments=$true)][string[]]$GameArgs)
$ErrorActionPreference = 'Stop'
try {
    $state = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'install-state.json') -Raw | ConvertFrom-Json
    if (-not (Test-Path -LiteralPath (Join-Path $state.root 'Omsi.exe'))) { throw 'Pasta do OMSI 2 nao encontrada. Execute Instalar_Beta.cmd.' }
    $env:OMSI_WHEELCHAIR_BETA = '1'
    $env:OMSI_WHEELCHAIR_AUTO = '1'
    $env:OMSI_NO_UPDATE = '1'
    $marker = Join-Path $PSScriptRoot 'changelog-beta2-visto.txt'
    if (-not (Test-Path -LiteralPath $marker)) {
        $news = Join-Path $PSScriptRoot 'CHANGELOG_INSTALADO.html'
        if (-not (Test-Path -LiteralPath $news)) { $news = Join-Path $PSScriptRoot 'CHANGELOG.html' }
        Start-Process $news
        'wheelchair-beta.2' | Set-Content -LiteralPath $marker
    }
    $logDir = Join-Path $PSScriptRoot 'logs'
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
    $stderr = Join-Path $logDir 'game-beta.log'
    $stdout = Join-Path $logDir 'game-beta-stdout.log'
    foreach ($file in @($stderr, $stdout)) {
        if (Test-Path -LiteralPath $file) { Copy-Item -LiteralPath $file -Destination ($file + '.anterior') -Force }
    }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'beta-manifest.json') -Destination $logDir -Force
    function Quote-NativeArgument([string]$Value) {
        return '"' + ($Value -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"'
    }
    $arguments = @('--root', $state.root)
    if ($GameArgs.Count -gt 0) { $arguments += $GameArgs } else { $arguments += @('--menu', '--passengers') }
    $commandLine = ($arguments | ForEach-Object { Quote-NativeArgument $_ }) -join ' '
    Write-Host ('Log desta sessao: ' + $stderr)
    $process = Start-Process -FilePath (Join-Path $PSScriptRoot 'openomsi.exe') -ArgumentList $commandLine -WorkingDirectory $PSScriptRoot -RedirectStandardError $stderr -RedirectStandardOutput $stdout -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw ('A beta terminou com codigo ' + $process.ExitCode + '. Execute Recolher_Logs.cmd.') }
} catch { Write-Host $_ -ForegroundColor Red; exit 1 }
