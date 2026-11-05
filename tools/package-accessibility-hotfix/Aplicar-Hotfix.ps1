param([string]$Destino)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
$PackageRoot = $PSScriptRoot
$Completed = $false
$Previous = @()
try {
    $Manifest = Get-Content -LiteralPath (Join-Path $PackageRoot 'hotfix.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    foreach ($File in $Manifest.files) {
        $Source = Join-Path $PackageRoot $File.path
        if (!(Test-Path -LiteralPath $Source -PathType Leaf)) { throw "Arquivo ausente: $($File.path)" }
        if ((Get-FileHash -LiteralPath $Source -Algorithm SHA256).Hash -ne $File.sha256) { throw "Arquivo incompleto ou alterado: $($File.path). Extraia novamente o ZIP." }
    }
    $GameRelative = 'OpenOmsi Coop 0.3.41\programa\openomsi.exe'
    if (!$Destino) {
        foreach ($Candidate in @($PackageRoot, (Split-Path -Parent $PackageRoot), (Get-Location).Path)) {
            if (Test-Path -LiteralPath (Join-Path $Candidate $GameRelative) -PathType Leaf) { $Destino = $Candidate; break }
        }
    }
    if (!$Destino) {
        $Dialog = New-Object System.Windows.Forms.FolderBrowserDialog
        $Dialog.Description = 'Selecione a pasta da Bridge onde ficam Setup.exe, app e OpenOmsi Coop 0.3.41.'
        $Dialog.ShowNewFolderButton = $false
        if ($Dialog.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { return }
        $Destino = $Dialog.SelectedPath
    }
    $Destino = [System.IO.Path]::GetFullPath($Destino)
    if (!(Test-Path -LiteralPath (Join-Path $Destino $GameRelative) -PathType Leaf)) { throw 'Esta pasta nao contem a instalacao da Bridge 2.2.3. Selecione a pasta onde ficam app e OpenOmsi Coop 0.3.41.' }
    $Running = Get-Process -Name 'openomsi','openomsi-launcher','openomsi-launcher-app','OpenOMSI_BCS_Bridge','HostAgent','CompanyHost' -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -and $_.Path.StartsWith($Destino.TrimEnd('\') + '\', [System.StringComparison]::OrdinalIgnoreCase) } catch { $false }
    }
    if ($Running) { throw 'Feche o jogo, o lobby, a Bridge e os programas de host desta instalacao antes de aplicar a Hotfix.' }
    $Operations = @(
        @{source='payload\openomsi.exe'; target=$GameRelative},
        @{source='payload\openomsi-launcher-app.exe'; target='OpenOmsi Coop 0.3.41\programa\openomsi-launcher-app.exe'},
        @{source='payload\SOURCE.json'; target='OpenOmsi Coop 0.3.41\source\SOURCE-2.2.3-hotfix.1.json'},
        @{source='payload\SOURCE.json'; target='OpenOmsi Coop 0.3.41\source\SOURCE-2.2.3.json'},
        @{source='payload\openomsi-source.tar.gz'; target='OpenOmsi Coop 0.3.41\source\openomsi-source-2.2.3-hotfix.1.tar.gz'},
        @{source='CHANGELOG-2.2.3-hotfix.1.md'; target='OpenOmsi Coop 0.3.41\CHANGELOG-2.2.3-hotfix.1.md'}
    )
    foreach ($Extra in @('app\server-dinamico-p16\openomsi.exe','PeDePeAdapter\openomsi.exe')) {
        if (Test-Path -LiteralPath (Join-Path $Destino $Extra) -PathType Leaf) { $Operations += @{source='payload\openomsi.exe'; target=$Extra} }
    }
    $BackupRoot = Join-Path $Destino ('backups\antes-2.2.3-hotfix.1-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $HashRelative = 'OpenOmsi Coop 0.3.41\SHA256.json'
    foreach ($Relative in (@($Operations | ForEach-Object {$_.target}) + @($HashRelative))) {
        $Target = Join-Path $Destino $Relative
        $Backup = Join-Path $BackupRoot $Relative
        $Existed = Test-Path -LiteralPath $Target -PathType Leaf
        if ($Existed) {
            New-Item -ItemType Directory -Path (Split-Path -Parent $Backup) -Force | Out-Null
            Copy-Item -LiteralPath $Target -Destination $Backup -Force
        }
        $Previous += @{target=$Target; backup=$Backup; existed=$Existed}
    }
    foreach ($Operation in $Operations) {
        $Source = Join-Path $PackageRoot $Operation.source
        $Target = Join-Path $Destino $Operation.target
        New-Item -ItemType Directory -Path (Split-Path -Parent $Target) -Force | Out-Null
        Copy-Item -LiteralPath $Source -Destination $Target -Force
        if ((Get-FileHash -LiteralPath $Target -Algorithm SHA256).Hash -ne (Get-FileHash -LiteralPath $Source -Algorithm SHA256).Hash) { throw "Falha ao conferir a copia de $($Operation.target)" }
    }
    $HashPath = Join-Path $Destino $HashRelative
    $Hashes = if (Test-Path -LiteralPath $HashPath -PathType Leaf) { Get-Content -LiteralPath $HashPath -Raw -Encoding UTF8 | ConvertFrom-Json } else { New-Object PSObject }
    foreach ($Operation in $Operations) {
        if ($Operation.target.StartsWith('OpenOmsi Coop 0.3.41\')) {
            $Name = $Operation.target.Substring('OpenOmsi Coop 0.3.41\'.Length).Replace('\','/')
            $Hash = (Get-FileHash -LiteralPath (Join-Path $Destino $Operation.target) -Algorithm SHA256).Hash.ToLowerInvariant()
            $Hashes | Add-Member -MemberType NoteProperty -Name $Name -Value $Hash -Force
        }
    }
    $Hashes | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $HashPath -Encoding UTF8
    $Completed = $true
    [System.Windows.Forms.MessageBox]::Show('Hotfix 1 aplicada. O changelog sera aberto agora. Voce ja pode abrir a Bridge e jogar.','OpenOMSI BCS Bridge 2.2.3 Hotfix 1') | Out-Null
    Start-Process notepad.exe -ArgumentList ('"' + (Join-Path $Destino 'OpenOmsi Coop 0.3.41\CHANGELOG-2.2.3-hotfix.1.md') + '"')
} catch {
    if (!$Completed) {
        foreach ($Item in $Previous) {
            try {
                if ($Item.existed) { Copy-Item -LiteralPath $Item.backup -Destination $Item.target -Force }
                elseif (Test-Path -LiteralPath $Item.target -PathType Leaf) { Remove-Item -LiteralPath $Item.target -Force }
            } catch { Write-Warning "Nao foi possivel restaurar $($Item.target). A copia anterior esta em $($Item.backup)." }
        }
    }
    [System.Windows.Forms.MessageBox]::Show($_.Exception.Message,'Nao foi possivel aplicar a Hotfix') | Out-Null
    exit 1
}
