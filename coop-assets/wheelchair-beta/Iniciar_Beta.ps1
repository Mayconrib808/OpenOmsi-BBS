param([Parameter(ValueFromRemainingArguments=$true)][string[]]$GameArgs)
$ErrorActionPreference = 'Stop'
try {
    $state = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'install-state.json') -Raw | ConvertFrom-Json
    if (-not (Test-Path -LiteralPath (Join-Path $state.root 'Omsi.exe'))) { throw 'Pasta do OMSI 2 nao encontrada. Execute Instalar_Beta.cmd.' }
    $env:OMSI_WHEELCHAIR_BETA = '1'
    $env:OMSI_WHEELCHAIR_AUTO = '1'
    $env:OMSI_NO_UPDATE = '1'
    if (-not (Test-Path -LiteralPath (Join-Path $PSScriptRoot 'changelog-beta1-visto.txt'))) {
        Start-Process (Join-Path $PSScriptRoot 'CHANGELOG.html')
        'wheelchair-beta.1' | Set-Content -LiteralPath (Join-Path $PSScriptRoot 'changelog-beta1-visto.txt')
    }
    Push-Location $PSScriptRoot
    try {
        if ($GameArgs.Count -gt 0) {
            & (Join-Path $PSScriptRoot 'openomsi.exe') --root $state.root @GameArgs
        } else {
            & (Join-Path $PSScriptRoot 'openomsi.exe') --root $state.root --menu --passengers
        }
        if ($LASTEXITCODE -ne 0) { throw "A beta terminou com codigo $LASTEXITCODE. Recolha o game.log." }
    } finally { Pop-Location }
} catch { Write-Host $_ -ForegroundColor Red; exit 1 }
