# Run the pinned, checksum-verified installer with Windows ShellExecute.
# This opens the normal interactive NSIS wizard and handles UAC where needed.
# No game is started, no DLL is injected, and no NVIDIA runtime is imported.
param([Parameter(Mandatory = $true)][string]$Installer)
$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $Installer -PathType Leaf)) {
    throw 'The verified installer could not be found'
}
if ([IO.Path]::GetFileName($Installer) -ne 'UniversalDLSS5-0.5.1-Setup-x64.exe') {
    throw 'Refusing to run an unexpected installer name'
}
# Verify once more immediately before ShellExecute (the cache is user-writable).
$Item = Get-Item -LiteralPath $Installer -Force
if (($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw 'Refusing a symbolic-link installer'
}
$ExpectedSha256 = '0ded01f5509a5cbb427806e1574d0f9ed21e200320191022677ec82e66916231'
$Actual = (Get-FileHash -LiteralPath $Installer -Algorithm SHA256).Hash.ToLowerInvariant()
if ($Actual -ne $ExpectedSha256) { throw 'DLSS 5 controller installer checksum changed before launch' }
Start-Process -FilePath $Installer -WorkingDirectory ([IO.Path]::GetDirectoryName($Installer)) -ErrorAction Stop
