$ErrorActionPreference = 'Stop'
$output = Join-Path $env:GITHUB_WORKSPACE 'build/release-2202'
$evidence = Join-Path $output 'evidence'
New-Item -ItemType Directory -Force $evidence | Out-Null
$scanDir = Join-Path $env:USERPROFILE 'Documents/Bridge2202Verification'
New-Item -ItemType Directory -Force $scanDir | Out-Null
Set-MpPreference -DisableArchiveScanning $false
if ((Get-MpPreference).DisableArchiveScanning) { throw 'Archive scanning could not be enabled.' }
$platform = Get-ChildItem "$env:ProgramData/Microsoft/Windows Defender/Platform/*/MpCmdRun.exe" -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
$scanner = if ($platform) { $platform.FullName } else { "$env:ProgramFiles/Windows Defender/MpCmdRun.exe" }
& $scanner -SignatureUpdate 2>&1 | Out-File (Join-Path $evidence 'defender-signature-update.log')
$status = Get-MpComputerStatus
if (-not $status.AntivirusEnabled) { throw 'Defender antivirus is unavailable.' }
$report = [ordered]@{ status = 'pending'; timestamp_utc = [DateTime]::UtcNow.ToString('o'); engine = $status.AMEngineVersion; signatures = $status.AntivirusSignatureVersion; archive_scanning_enabled = $true; files = @() }
foreach ($name in @('OpenOmsi_BBS_2.2.0.2_Setup_e_Coop8.zip', 'Atualizar-Bridge.exe')) {
    $target = Join-Path $scanDir $name
    Copy-Item (Join-Path $output $name) $target
    $hash = (Get-FileHash $target -Algorithm SHA256).Hash.ToLowerInvariant()
    $log = Join-Path $evidence ($name + '-defender.log')
    & $scanner -Scan -ScanType 3 -File $target -DisableRemediation 2>&1 | Tee-Object -FilePath $log
    $code = $LASTEXITCODE
    $text = Get-Content $log -Raw
    $passed = $code -eq 0 -and $text -match 'found no threats|no threats found' -and $text -notmatch 'skipped'
    $report.files += [ordered]@{ name = $name; sha256 = $hash; exit_code = $code; passed = $passed; scope = 'exact file' }
    if (-not $passed) {
        $report.status = 'requires_review'
        $report | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $evidence 'defender-2202.json')
        throw ('Defender scan did not pass for ' + $name)
    }
}
$report.status = 'passed'
$report | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $evidence 'defender-2202.json')
