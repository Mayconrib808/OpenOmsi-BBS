param([switch]$NoOpen)
$ErrorActionPreference = 'Stop'
try {
    $logDir = Join-Path $PSScriptRoot 'logs'
    if (-not (Test-Path -LiteralPath (Join-Path $logDir 'game-beta.log'))) { throw 'Inicie pelo INICIAR_BETA.cmd, reproduza o problema e feche o jogo primeiro.' }
    $files = @('game-beta.log', 'game-beta-stdout.log', 'beta-manifest.json') | ForEach-Object { Join-Path $logDir $_ } | Where-Object { Test-Path -LiteralPath $_ }
    $zip = Join-Path $PSScriptRoot 'Logs-Beta2.zip'
    Compress-Archive -LiteralPath $files -DestinationPath $zip -Force
    Write-Host ('Envie este arquivo: ' + $zip) -ForegroundColor Green
    if (-not $NoOpen) { Start-Process explorer.exe -ArgumentList ('/select,"' + $zip + '"') }
} catch { Write-Host $_ -ForegroundColor Red; exit 1 }
