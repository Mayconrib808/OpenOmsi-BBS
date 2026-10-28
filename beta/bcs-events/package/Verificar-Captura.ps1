param(
    [string]$CaptureDirectory = (Join-Path $env:LOCALAPPDATA 'OpenOmsi-BBS\EventosBeta'),
    [switch]$OpenFolder
)
$ErrorActionPreference = 'Stop'
New-Item -ItemType Directory -Path $CaptureDirectory -Force | Out-Null
$states = @()
foreach ($file in (Get-ChildItem -LiteralPath $CaptureDirectory -Filter 'bcs-events-*.json' -File)) {
    try {
        $state = Get-Content -LiteralPath $file.FullName -Raw | ConvertFrom-Json
        if ($state.probe_version -ne '0.1.2') { continue }
        $process = Get-Process -Id ([int]$state.pid) -ErrorAction SilentlyContinue
        if ($process -and ([DateTime]::UtcNow - $file.LastWriteTimeUtc).TotalSeconds -lt 20) { $states += $state }
    } catch { }
}
if (@($states | Where-Object status -eq 'compatible').Count -gt 0) {
    $message = 'CAPTURA ATIVA: BCS reconhecido e status recente gravado. Nao precisa de evento para confirmar.'
} elseif (@($states | Where-Object { $_.status -in @('unsupported_bcs_build', 'capture_error', 'observer_failed') }).Count -gt 0) {
    $message = 'CAPTURA INDISPONIVEL: o agente informou incompatibilidade ou erro. Envie os JSONs e diagnosticos; nao espere outro evento.'
} elseif ($states.Count -gt 0) {
    $message = 'AGUARDANDO BCS: o agente esta carregado, mas ainda nao reconheceu o gerenciador de eventos. Confira novamente apos iniciar a viagem.'
} else {
    $message = 'CAPTURA NAO CONFIRMADA: nao ha status recente de um agente ativo. Abrir-Capturas.bat so verifica e abre a pasta; nao inicia a captura. Envie diagnostico-inicio-*.txt e a pasta diagnostico.'
}
Write-Host $message
Set-Content -LiteralPath (Join-Path $CaptureDirectory 'LEIA-STATUS-DA-CAPTURA.txt') -Value ((Get-Date -Format o) + "`r`n" + $message) -Encoding UTF8
if ($OpenFolder) { Start-Process -FilePath 'explorer.exe' -ArgumentList ('"' + $CaptureDirectory + '"') | Out-Null }
