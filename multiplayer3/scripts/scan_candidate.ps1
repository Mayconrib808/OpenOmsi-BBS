param([Parameter(Mandatory=$true)][string]$PackagePath)
$ErrorActionPreference='Stop'
$source=(Resolve-Path -LiteralPath $PackagePath).Path
$report=[ordered]@{package=(Split-Path $source -Leaf);sha256=(Get-FileHash $source -Algorithm SHA256).Hash.ToLowerInvariant();status='not_scanned';scans=@();utc=[DateTime]::UtcNow.ToString('o')}
$output=Join-Path (Split-Path $source -Parent) 'DEFENDER-REPORT.json'
try {
 $scanner=@(Get-ChildItem "$env:ProgramData\Microsoft\Windows Defender\Platform" -Filter MpCmdRun.exe -Recurse -ErrorAction SilentlyContinue|Sort-Object FullName -Descending|Select-Object -First 1)
 $path=if($scanner.Count){$scanner[0].FullName}else{Join-Path $env:ProgramFiles 'Windows Defender\MpCmdRun.exe'}
 $service=Get-Service WinDefend -ErrorAction SilentlyContinue
 if(!(Test-Path $path) -or !$service -or $service.Status -ne 'Running') {
  $report.status='scanner_unavailable';Write-Host 'Defender unavailable on this runner; no clean-scan claim.';return
 }
 $status=Get-MpComputerStatus
 $report.signature=$status.AntivirusSignatureVersion
 $folder=Join-Path $env:RUNNER_TEMP 'bbs-v3-candidate-scan'
 New-Item -ItemType Directory -Force $folder|Out-Null
 Expand-Archive -LiteralPath $source -DestinationPath $folder -Force
 $targets=@($source)+@(Get-ChildItem $folder -Recurse -File|Where-Object {$_.Extension -in '.exe','.dll'}|ForEach-Object {$_.FullName})
 $failed=$false
 foreach($target in $targets) {
  $result=& $path -Scan -ScanType 3 -File $target -DisableRemediation 2>&1|Out-String
  $code=$LASTEXITCODE
  $report.scans+=@{file=(Split-Path $target -Leaf);sha256=(Get-FileHash $target -Algorithm SHA256).Hash.ToLowerInvariant();exit_code=$code;output=$result}
  Write-Host "Defender result: $(Split-Path $target -Leaf), exit $code"
  Write-Host $result
  if($code -ne 0 -and $result -match '0x800106ba|0x800706ba') {
   # Repair only the ephemeral CI scanner's service state, never weaken protection.
   try {
    & $path -WdEnable 2>&1|Out-String|Write-Host
    Start-Service WinDefend
    Update-MpSignature
    $result=& $path -Scan -ScanType 3 -File $target -DisableRemediation 2>&1|Out-String
    $code=$LASTEXITCODE
    $report.scans+=@{file=(Split-Path $target -Leaf);retry=$true;exit_code=$code;output=$result}
    Write-Host $result
   } catch {Write-Host $_}
   if($code -ne 0 -and $result -match '0x800106ba|0x800706ba') {
    $report.status='scanner_unavailable_rpc';Write-Host 'Defender RPC service unavailable; no file verdict was produced.';return
   }
  }
  if($code -ne 0){$failed=$true}
 }
 if($failed){$report.status='detection_or_scan_error';throw 'Defender detection or scan error: publication blocked.'}
 $report.status='clean_on_this_runner'
} finally {
 try {$report.threats=@(Get-MpThreat|Select-Object ThreatName,IsActive,ThreatID)}catch{}
 $report|ConvertTo-Json -Depth 8|Write-Host
 $report|ConvertTo-Json -Depth 8|Set-Content -LiteralPath $output -Encoding utf8
}
