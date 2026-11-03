param([string]$OmsiRoot, [switch]$Uninstall, [switch]$NoOpen)
$ErrorActionPreference = 'Stop'
try {
    $statePath = Join-Path $PSScriptRoot 'install-state.json'
    if ($Uninstall) {
        $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
        foreach ($item in $state.profiles) {
            if (Test-Path -LiteralPath $item.destination) {
                $hash = (Get-FileHash -LiteralPath $item.destination -Algorithm SHA256).Hash.ToLowerInvariant()
                if ($hash -ne $item.installed_hash) {
                    Write-Host "Perfil alterado apos a instalacao; preservado: $($item.destination)"
                    continue
                }
                if ($item.backup -and (Test-Path -LiteralPath $item.backup)) {
                    Copy-Item -LiteralPath $item.backup -Destination $item.destination -Force
                } else { Remove-Item -LiteralPath $item.destination }
            }
        }
        Write-Host 'Perfis da beta removidos ou restaurados. A versao estavel continua no local original.'
        Write-Host 'Voce pode apagar a pasta BridgeWheelchairBeta depois de fechar esta janela.'
        exit 0
    }
    if (-not $OmsiRoot) {
        Add-Type -AssemblyName System.Windows.Forms
        $dialog = New-Object System.Windows.Forms.FolderBrowserDialog
        $dialog.Description = 'Escolha a pasta OMSI 2 que contem Omsi.exe, maps e Vehicles'
        if ($dialog.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 0 }
        $OmsiRoot = $dialog.SelectedPath
    }
    $OmsiRoot = (Resolve-Path -LiteralPath $OmsiRoot).Path
    foreach ($required in @('Omsi.exe', 'maps', 'Vehicles')) {
        if (-not (Test-Path -LiteralPath (Join-Path $OmsiRoot $required))) { throw "Falta $required na pasta escolhida." }
    }
    $manifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'beta-manifest.json') -Raw | ConvertFrom-Json
    $hash = (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot 'openomsi.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hash -ne $manifest.native_sha256) { throw 'O executavel nao corresponde ao pacote. Extraia novamente o ZIP.' }
    $target = Join-Path $OmsiRoot 'BridgeWheelchairBeta'
    $oldState = $null
    $oldStatePath = Join-Path $target 'install-state.json'
    if (Test-Path -LiteralPath $oldStatePath) { $oldState = Get-Content -LiteralPath $oldStatePath -Raw | ConvertFrom-Json }
    New-Item -ItemType Directory -Path $target -Force | Out-Null
    if ($PSScriptRoot -ne $target) {
        Get-ChildItem -LiteralPath $PSScriptRoot | Where-Object { $_.Name -notin @('install-state.json', 'backups', 'logs') } | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination $target -Recurse -Force
        }
    }
    $assetCandidates = @((Join-Path $OmsiRoot 'bcs-assets'))
    Get-ChildItem -LiteralPath $OmsiRoot -Directory -Filter 'OpenOmsi Coop*' | ForEach-Object {
        $assetCandidates += (Join-Path $_.FullName 'programa/bcs-assets')
    }
    $baseAssets = $assetCandidates | Where-Object { Test-Path -LiteralPath $_ -PathType Container } | Select-Object -First 1
    if ($baseAssets) {
        $assetTarget = Join-Path $target 'bcs-assets'
        New-Item -ItemType Directory -Path $assetTarget -Force | Out-Null
        Get-ChildItem -LiteralPath $baseAssets | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination $assetTarget -Recurse -Force
        }
    }
    $records = @()
    $vehicles = Join-Path $OmsiRoot 'Vehicles'
    $folders = Get-ChildItem -LiteralPath $vehicles -Directory | Where-Object { $_.Name -eq '2007 Marcopolo Torino OF 1722M' }
    foreach ($folder in $folders) {
        $dest = Join-Path $folder.FullName 'wheelchair.json'
        $prior = @($oldState.profiles | Where-Object { $_.destination -eq $dest }) | Select-Object -First 1
        $backup = if ($prior) { $prior.backup } else { $null }
        $changedSincePrior = $prior -and (Test-Path -LiteralPath $dest) -and ((Get-FileHash -LiteralPath $dest -Algorithm SHA256).Hash.ToLowerInvariant() -ne $prior.installed_hash)
        if ((Test-Path -LiteralPath $dest) -and (-not $prior -or $changedSincePrior)) {
            $backupDir = Join-Path $target 'backups'
            New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
            $backup = Join-Path $backupDir ('wheelchair-' + [Guid]::NewGuid().ToString('N') + '.json')
            Copy-Item -LiteralPath $dest -Destination $backup
        }
        $profile = Join-Path $target 'profiles/torino2007.json'
        Copy-Item -LiteralPath $profile -Destination $dest -Force
        $installed = (Get-FileHash -LiteralPath $dest -Algorithm SHA256).Hash.ToLowerInvariant()
        $records += @{ destination=$dest; backup=$backup; installed_hash=$installed }
    }
    @{ root=$OmsiRoot; version=$manifest.version; profiles=$records } | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $target 'install-state.json') -Encoding UTF8
    if (-not $NoOpen) { Start-Process (Join-Path $target 'CHANGELOG.html') }
    if ($records.Count -eq 0) { Write-Host 'Torino original nao encontrado. Os outros onibus precisam de um perfil medido.' -ForegroundColor Yellow }
    Write-Host "Beta instalada. Abra: $target\INICIAR_BETA.cmd" -ForegroundColor Green
} catch { Write-Host $_ -ForegroundColor Red; exit 1 }
