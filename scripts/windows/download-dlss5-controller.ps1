# Installs NO NVIDIA DLLs. Downloads one user-authorized third-party controller
# installer from a fixed GitHub release, with a pinned SHA-256. The installer
# itself stays interactive; nothing is injected into the game by this script.
param([Parameter(Mandatory = $true)][string]$Destination)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$Url = 'https://github.com/MotionflowOffical/UniversalDLSS5/releases/download/v0.5.1/UniversalDLSS5-0.5.1-Setup-x64.exe'
$ExpectedSha256 = '0ded01f5509a5cbb427806e1574d0f9ed21e200320191022677ec82e66916231'
$MinimumBytes = 2000000
$MaximumBytes = 8000000
if (-not [IO.Path]::IsPathRooted($Destination)) { throw 'Installer destination must be absolute' }
$Parent = [IO.Path]::GetDirectoryName($Destination)
if (-not $Parent) { throw 'Missing installer cache directory' }
New-Item -ItemType Directory -Path $Parent -Force -ErrorAction Stop | Out-Null
function Test-VerifiedInstaller([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    $Item = Get-Item -LiteralPath $Path -Force
    if (($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Refusing symlinked DLSS 5 installer cache' }
    if ($Item.Length -lt $MinimumBytes -or $Item.Length -gt $MaximumBytes) { return $false }
    $Actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    return $Actual -eq $ExpectedSha256
}
if (Test-VerifiedInstaller $Destination) {
    Write-Output 'Previously downloaded UniversalDLSS5 installer verified.'
    exit 0
}
$Temp = Join-Path ([IO.Path]::GetTempPath()) ('openomsi-dlss5-' + [guid]::NewGuid().ToString('N') + '.exe')
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $Url -OutFile $Temp -UseBasicParsing -MaximumRedirection 5 -TimeoutSec 300 -Headers @{ 'User-Agent' = 'openOMSI-Launcher' }
    if (-not (Test-VerifiedInstaller $Temp)) { throw 'UniversalDLSS5 GitHub release file failed the pinned SHA-256/size validation' }
    Move-Item -LiteralPath $Temp -Destination $Destination -Force -ErrorAction Stop
    if (-not (Test-VerifiedInstaller $Destination)) { throw 'Cached installer failed the post-download validation' }
    Write-Output 'Verified UniversalDLSS5 v0.5.1 installer downloaded from the official project release.'
} finally {
    if (Test-Path -LiteralPath $Temp) { Remove-Item -LiteralPath $Temp -Force -ErrorAction SilentlyContinue }
}
