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
Start-Process -FilePath $Installer -WorkingDirectory ([IO.Path]::GetDirectoryName($Installer)) -ErrorAction Stop
