# Scan the exact candidate ZIP and its six executables before publication.
# This disposable runner only strengthens protection: no allow rules, scan
# exclusions, disabled antivirus or remediation of detections are requested.
[CmdletBinding()]
param([Parameter(Mandatory = $true)][string] $PackagePath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
$reportDirectory = Join-Path $env:RUNNER_TEMP 'openomsi-defender-report'
# Hosted CI workspaces may be excluded by the image; scan outside the workspace.
$scanDirectory = Join-Path (Join-Path $env:SystemDrive 'OpenOmsiDefenderInput') ([Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $reportDirectory, $scanDirectory | Out-Null
$report = [ordered]@{
    package = [System.IO.Path]::GetFileName($PackagePath)
    started_utc = [DateTime]::UtcNow.ToString('o')
    status = 'inconclusive'
    scans = @()
    error = $null
}
$executables = @('Setup.exe', 'HostAgent.exe', 'CompanyHost.exe',
    'app/OpenOMSI_BCS_Bridge.exe', 'app/compat/Omsi.exe', 'app/compat/omsi-plugin-host32.exe')

try {
    $source = (Resolve-Path -LiteralPath $PackagePath).Path
    $checksum = (Get-Content -Raw -LiteralPath ($source + '.sha256')).Trim()
    $parts = $checksum -split '  ', 2
    if ($parts.Count -ne 2 -or $parts[0] -cnotmatch '^[a-f0-9]{64}$' -or $parts[1] -cne $report.package) {
        throw 'Invalid package SHA-256 sidecar.'
    }
    $report.expected_zip_sha256 = $parts[0]
    $sourceHash = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($sourceHash -ne $report.expected_zip_sha256) { throw 'Candidate ZIP checksum mismatch.' }

    $platform = Join-Path $env:ProgramData 'Microsoft\Windows Defender\Platform'
    $scanner = @(Get-ChildItem $platform -Filter MpCmdRun.exe -Recurse -ErrorAction SilentlyContinue |
        Sort-Object FullName -Descending | Select-Object -First 1)
    $scannerPath = if ($scanner.Count) { $scanner[0].FullName } else {
        Join-Path $env:ProgramFiles 'Windows Defender\MpCmdRun.exe'
    }
    if (-not (Test-Path -LiteralPath $scannerPath)) { throw 'Microsoft Defender scanner is unavailable.' }
    $report.scanner_path = $scannerPath
    if ((Get-Service WinDefend).Status -ne 'Running') {
        & $scannerPath -WdEnable 2>&1 | Out-String | Write-Host
        Start-Service WinDefend
    }
    $report.defender_preferences_before = Get-MpPreference | Select-Object MAPSReporting,
        SubmitSamplesConsent, DisableBlockAtFirstSeen, DisableRealtimeMonitoring, DisableIOAVProtection
    Set-MpPreference -MAPSReporting Advanced -SubmitSamplesConsent SendSafeSamples `
        -DisableBlockAtFirstSeen $false -DisableRealtimeMonitoring $false -DisableIOAVProtection $false
    $preferences = Get-MpPreference
    $report.defender_preferences_after = $preferences | Select-Object MAPSReporting,
        SubmitSamplesConsent, DisableBlockAtFirstSeen, DisableRealtimeMonitoring, DisableIOAVProtection
    Update-MpSignature -UpdateSource MMPC
    $computerStatus = Get-MpComputerStatus
    $report.defender = $computerStatus | Select-Object AMServiceEnabled, AntivirusEnabled,
        AMRunningMode, RealTimeProtectionEnabled, AMProductVersion, AMEngineVersion,
        AntivirusSignatureVersion, AntivirusSignatureLastUpdated
    $report.defender | Format-List | Out-String | Write-Host
    if (-not $computerStatus.AMServiceEnabled -or -not $computerStatus.AntivirusEnabled `
        -or -not $computerStatus.RealTimeProtectionEnabled -or $preferences.DisableIOAVProtection `
        -or $preferences.DisableBlockAtFirstSeen -or [int]$preferences.MAPSReporting -eq 0) {
        throw 'Defender real-time/download/cloud protection is unavailable; no clean result can be claimed.'
    }
    $cloudOutput = & $scannerPath -ValidateMapsConnection 2>&1 | Out-String
    $report.cloud_connection_exit_code = $LASTEXITCODE
    $cloudOutput | Set-Content (Join-Path $reportDirectory 'cloud-connection.txt') -Encoding utf8
    Write-Host $cloudOutput
    if ($report.cloud_connection_exit_code -ne 0) { throw 'Defender cloud validation failed; publication is blocked.' }

    $zipPath = Join-Path $scanDirectory $report.package
    Copy-Item -LiteralPath $source -Destination $zipPath
    # Apply the Internet marker to the candidate; this is not a browser-download test.
    Set-Content -LiteralPath $zipPath -Stream Zone.Identifier -Encoding ascii `
        -Value "[ZoneTransfer]`r`nZoneId=3`r`n"
    $report.actual_zip_sha256 = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($report.actual_zip_sha256 -ne $report.expected_zip_sha256) { throw 'Scanned ZIP differs from candidate.' }

    function Invoke-PackageScan([string] $Path, [string] $Label) {
        # This custom-scan mode ignores file exclusions and includes archives.
        # Findings are reported, and no executable from the package is run here.
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
        if ($code -ne 0) {
            $report.status = 'detections_or_scan_errors'
            throw "Defender reported a detection or scan error for $Label. Publication is blocked."
        }
    }
    Invoke-PackageScan $zipPath 'candidate-zip'

    $archive = [System.IO.Compression.ZipFile]::OpenRead($zipPath)
    try {
        $reader = [System.IO.StreamReader]::new($archive.GetEntry('docs/SHA256.txt').Open())
        try { $manifest = $reader.ReadToEnd() } finally { $reader.Dispose() }
        $hashes = @{}
        foreach ($line in ($manifest -split '\r?\n')) {
            if (-not $line) { continue }
            if ($line -cnotmatch '^([a-f0-9]{64})  (.+)$' -or $hashes.ContainsKey($Matches[2])) {
                throw 'Invalid or duplicate package manifest entry.'
            }
            $hashes[$Matches[2]] = $Matches[1]
        }
        $actualExecutables = @($archive.Entries | Where-Object { $_.FullName.EndsWith('.exe', [StringComparison]::OrdinalIgnoreCase) } |
            ForEach-Object { $_.FullName })
        if ($actualExecutables.Count -ne 6 -or @(Compare-Object $executables $actualExecutables).Count) {
            throw 'Package must contain exactly the six expected executables.'
        }
        # Only fixed, allowlisted paths are extracted; compare all six hashes
        # against the complete package manifest verified again by the publisher.
        foreach ($relativePath in $executables) {
            $entry = $archive.GetEntry($relativePath)
            if ($null -eq $entry -or -not $hashes.ContainsKey($relativePath)) { throw "Missing executable: $relativePath" }
            $target = Join-Path $scanDirectory $relativePath
            New-Item -ItemType Directory -Force (Split-Path $target -Parent) | Out-Null
            $inputStream = $entry.Open()
            try {
                $outputStream = [System.IO.File]::Create($target)
                try { $inputStream.CopyTo($outputStream) } finally { $outputStream.Dispose() }
            } finally { $inputStream.Dispose() }
            $actual = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($actual -ne $hashes[$relativePath]) { throw "Executable hash mismatch: $relativePath" }
            Invoke-PackageScan $target $relativePath
        }
    } finally { $archive.Dispose() }
    $report.status = 'clean_in_this_environment_not_a_microsoft_false_positive_ruling'
} catch {
    $report.error = $_.Exception.Message
    Write-Host "Package scan finding or limitation: $($report.error)"
    throw
} finally {
    try {
        $report.threats = @(Get-MpThreat | Select-Object ThreatID, ThreatName, IsActive)
        $report.detections = @(Get-MpThreatDetection | Select-Object ThreatID, Resources,
            InitialDetectionTime, LastThreatStatusChangeTime, ActionSuccess)
    } catch { $report.threat_history_error = $_.Exception.Message }
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report | ConvertTo-Json -Depth 12 | Set-Content (Join-Path $reportDirectory 'report.json') -Encoding utf8
    Write-Host "Package scan status: $($report.status)"
}
