# Diagnostic only: scans the already published bytes, never executes the bridge,
# never disables antivirus, and never creates allow rules or scan exclusions.
# A clean result in this runner is not a false-positive ruling by Microsoft.
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

$reportDirectory = Join-Path $env:RUNNER_TEMP 'openomsi-defender-report'
# Keep downloads outside the CI workspace, which hosted images can exclude.
$scanDirectory = Join-Path $env:SystemDrive 'OpenOmsiDefenderInput'
New-Item -ItemType Directory -Force $reportDirectory, $scanDirectory | Out-Null
$report = [ordered]@{
    release = 'v2.0.3-dev.1'
    expected_zip_sha256 = 'a939ee3b38b90c96525af890946d4c8c4d3504712a001629648568f6ff90586d'
    started_utc = [DateTime]::UtcNow.ToString('o')
    status = 'inconclusive'
    scans = @()
    error = $null
}
$expectedExecutables = [ordered]@{
    'app/compat/omsi-plugin-host32.exe' = '1c7f1f25bd18edb2b010ec42de2e34db5f3fc8ed2b1d948003635528b8a11a1a'
    'app/compat/Omsi.exe' = 'a24fab80d68b95c58d07df5918502f025089763a971d57d7d54a1f8156b67339'
    'app/OpenOMSI_BCS_Bridge.exe' = '606729d6309af0f917a33371508c008ff5e8715c6a76820efd2cfe6da6584bd8'
    'CompanyHost.exe' = '7dd724f442d5d8522b9bf33c3156c7b01654b8b0445858f6f825883e499ec983'
    'HostAgent.exe' = '6288c19080c65c2b58b5d93e7b978b79df20845858f5088198398db1250f1e49'
    'Setup.exe' = '305ef50017bda69324399be3a30e81dc75fc882b826bd6afa16f5ae7b747165b'
}

try {
    $platform = Join-Path $env:ProgramData 'Microsoft\Windows Defender\Platform'
    $scanner = @(Get-ChildItem $platform -Filter MpCmdRun.exe -Recurse -ErrorAction SilentlyContinue |
        Sort-Object FullName -Descending | Select-Object -First 1)
    $scannerPath = if ($scanner.Count) { $scanner[0].FullName } else {
        Join-Path $env:ProgramFiles 'Windows Defender\MpCmdRun.exe'
    }
    if (-not (Test-Path -LiteralPath $scannerPath)) { throw 'Microsoft Defender scanner is unavailable.' }
    $report.scanner_path = $scannerPath

    $service = Get-Service WinDefend
    if ($service.Status -ne 'Running') {
        # Enable the scanner only in this disposable Windows runner.
        & $scannerPath -WdEnable 2>&1 | Out-String | Write-Host
        Start-Service WinDefend
    }
    # Hosted runners can ship with real-time/cloud protection switched off.
    # Strengthen protection in this disposable runner before downloading the
    # public release, so a download-triggered cloud detection can be observed.
    $report.defender_preferences_before = Get-MpPreference | Select-Object MAPSReporting,
        SubmitSamplesConsent, DisableBlockAtFirstSeen, DisableRealtimeMonitoring, DisableIOAVProtection
    Set-MpPreference -MAPSReporting Advanced -SubmitSamplesConsent SendSafeSamples `
        -DisableBlockAtFirstSeen $false -DisableRealtimeMonitoring $false -DisableIOAVProtection $false
    $report.defender_preferences_after = Get-MpPreference | Select-Object MAPSReporting,
        SubmitSamplesConsent, DisableBlockAtFirstSeen, DisableRealtimeMonitoring, DisableIOAVProtection
    $report.defender_preferences_after | Format-List | Out-String | Write-Host
    Update-MpSignature -UpdateSource MMPC
    $computerStatus = Get-MpComputerStatus
    $report.defender = $computerStatus | Select-Object AMServiceEnabled, AntivirusEnabled,
        AMRunningMode, RealTimeProtectionEnabled, AMProductVersion, AMEngineVersion,
        AntivirusSignatureVersion, AntivirusSignatureLastUpdated
    $report.defender | Format-List | Out-String | Write-Host
    if (-not $computerStatus.AMServiceEnabled -or -not $computerStatus.AntivirusEnabled `
        -or -not $computerStatus.RealTimeProtectionEnabled) {
        throw 'Defender is not available in active antivirus mode; no clean result can be claimed.'
    }
    $cloudOutput = & $scannerPath -ValidateMapsConnection 2>&1 | Out-String
    $report.cloud_connection_exit_code = $LASTEXITCODE
    $cloudOutput | Set-Content (Join-Path $reportDirectory 'cloud-connection.txt') -Encoding utf8
    Write-Host $cloudOutput

    $zipPath = Join-Path $scanDirectory 'OpenOmsi.+.BBS.2.0.3-dev.1.zip'
    Invoke-WebRequest -Uri 'https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v2.0.3-dev.1/OpenOmsi.%2B.BBS.2.0.3-dev.1.zip' -OutFile $zipPath
    # Preserve the Internet-zone marker normally applied by a browser download.
    Set-Content -LiteralPath $zipPath -Stream Zone.Identifier -Encoding ascii `
        -Value "[ZoneTransfer]`r`nZoneId=3`r`n"
    $zipHash = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $report.actual_zip_sha256 = $zipHash
    if ($zipHash -ne $report.expected_zip_sha256) { throw 'Published ZIP hash differs from the verified native release.' }
    Write-Host "Verified published ZIP: $zipHash"

    # MpCmdRun -DisableRemediation deliberately ignores exclusions and scans
    # archive contents. Findings stay in output; bridge programs are not run.
    function Invoke-ReleaseScan([string] $Path, [string] $Label) {
        $output = & $scannerPath -Scan -ScanType 3 -File $Path -DisableRemediation 2>&1 | Out-String
        $code = $LASTEXITCODE
        $logName = ($Label -replace '[^a-zA-Z0-9.-]', '_') + '.txt'
        $output | Set-Content (Join-Path $reportDirectory $logName) -Encoding utf8
        Write-Host "Defender result for ${Label}: exit $code"
        Write-Host $output
        $report.scans += [ordered]@{
            item = $Label
            sha256 = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
            exit_code = $code
            output = $output
        }
    }
    Invoke-ReleaseScan $zipPath 'published-zip'

    # Extract only the six exact allowlisted package entries. Paths are fixed
    # here and SHA-256 checked; ZIP-provided paths cannot escape scanDirectory.
    $archive = [System.IO.Compression.ZipFile]::OpenRead($zipPath)
    try {
        foreach ($relativePath in $expectedExecutables.Keys) {
            $entry = $archive.GetEntry($relativePath)
            if ($null -eq $entry) { throw "Missing expected executable: $relativePath" }
            $target = Join-Path $scanDirectory $relativePath
            New-Item -ItemType Directory -Force (Split-Path $target -Parent) | Out-Null
            $inputStream = $entry.Open()
            try {
                $outputStream = [System.IO.File]::Create($target)
                try { $inputStream.CopyTo($outputStream) } finally { $outputStream.Dispose() }
            } finally { $inputStream.Dispose() }
            $actual = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($actual -ne $expectedExecutables[$relativePath]) { throw "Executable hash mismatch: $relativePath" }
            Invoke-ReleaseScan $target $relativePath
        }
    } finally { $archive.Dispose() }

    if (@($report.scans | Where-Object { $_.exit_code -ne 0 }).Count) {
        $report.status = 'detections_or_scan_errors'
        throw 'At least one scan reported a detection or error. Consult per-file scanner output.'
    }
    if ($report.cloud_connection_exit_code -ne 0) {
        $report.status = 'local_scans_clean_cloud_unavailable'
        throw 'Local scans completed, but cloud validation failed; this cannot reproduce cloud-assisted download scanning.'
    }
    $report.status = 'clean_in_this_environment_not_a_microsoft_false_positive_ruling'
} catch {
    $report.error = $_.Exception.Message
    Write-Host "Diagnostic limitation or finding: $($report.error)"
    throw
} finally {
    try {
        $report.threats = @(Get-MpThreat | Select-Object ThreatID, ThreatName, IsActive)
        $report.detections = @(Get-MpThreatDetection | Select-Object ThreatID, Resources,
            InitialDetectionTime, LastThreatStatusChangeTime, ActionSuccess)
    } catch { $report.threat_history_error = $_.Exception.Message }
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report | ConvertTo-Json -Depth 12 | Set-Content (Join-Path $reportDirectory 'report.json') -Encoding utf8
    Write-Host "Diagnostic status: $($report.status)"
}
