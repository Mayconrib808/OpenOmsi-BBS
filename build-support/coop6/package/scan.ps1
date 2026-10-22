param([ValidateSet('files','zip')][string]$Mode)
$ErrorActionPreference = 'Stop'
$package = "$env:GITHUB_WORKSPACE/final/OpenOMSI_Coop_0.3.38_coop.6_Portatil"
$target = if ($Mode -eq 'files') { $package } else { "$env:GITHUB_WORKSPACE/final/Coop_6_Windows_x64.zip" }
$reportPath = if ($Mode -eq 'files') { "$package/validation/defender-files.json" } else { "$env:GITHUB_WORKSPACE/final/defender-final.json" }
$logPath = if ($Mode -eq 'files') { "$package/validation/defender-files.log" } else { "$env:GITHUB_WORKSPACE/final/defender-final.log" }
$report = [ordered]@{ status = 'unavailable'; scope = $Mode; timestamp_utc = [DateTime]::UtcNow.ToString('o') }
try {
    $service = Get-Service WinDefend -ErrorAction Stop
    if ($service.Status -ne 'Running') { Start-Service WinDefend -ErrorAction Stop }
    $platform = Get-ChildItem "$env:ProgramData/Microsoft/Windows Defender/Platform/*/MpCmdRun.exe" -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
    $scanner = if ($platform) { $platform.FullName } else { "$env:ProgramFiles/Windows Defender/MpCmdRun.exe" }
    if (-not (Test-Path $scanner)) { throw 'Microsoft Defender command-line scanner not present.' }
    & $scanner -SignatureUpdate 2>&1 | Out-File "$env:RUNNER_TEMP/coop6-defender-update.log"
    $status = Get-MpComputerStatus -ErrorAction Stop
    $report.engine_version = $status.AMEngineVersion
    $report.signature_version = $status.AntivirusSignatureVersion
    $report.antivirus_enabled = $status.AntivirusEnabled
    if ($Mode -eq 'zip') { $report.zip_sha256 = (Get-FileHash $target -Algorithm SHA256).Hash.ToLowerInvariant() }
    & $scanner -Scan -ScanType 3 -File $target -DisableRemediation 2>&1 | Tee-Object -FilePath $logPath
    $code = $LASTEXITCODE
    $report.exit_code = $code
    $scanText = Get-Content $logPath -Raw
    if ($code -eq 0 -and $scanText -match 'found no threats|no threats found') { $report.status = 'passed' }
    elseif ($code -ne 0) { $report.status = 'scan_error_or_detection' }
    else { $report.status = 'review_log' }
} catch { $report.error = $_.Exception.Message }
$report | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $reportPath
if ($report.status -eq 'scan_error_or_detection') { throw 'Defender scan requires investigation. See report and log.' }
