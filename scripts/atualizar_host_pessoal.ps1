# Host-only personal update for Bridge 2.2.0.2 with verified transaction backups.
[CmdletBinding()]
param([string]$Destination, [string]$PreferredDestination, [switch]$NonInteractive, [switch]$NoLaunch)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

function Set-JsonField {
    param($Document, [string]$Name, $Value)
    $property = $Document.PSObject.Properties[$Name]
    if ($null -eq $property) {
        Add-Member -InputObject $Document -MemberType NoteProperty -Name $Name -Value $Value
    } else {
        $property.Value = $Value
    }
}

function Get-PersonalDataRoot {
    param([string]$SourceRoot)
    foreach ($candidate in @($env:LOCALAPPDATA, $env:APPDATA)) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and [IO.Path]::IsPathRooted($candidate)) {
            return (Join-Path -Path $candidate -ChildPath 'OpenOmsi-BBS')
        }
    }
    return (Join-Path -Path $SourceRoot -ChildPath 'user-settings')
}

function Test-PathInside {
    param([string]$Path, [string]$Directory)
    $root = [IO.Path]::GetFullPath($Directory).TrimEnd([char[]]'\/')
    $full = [IO.Path]::GetFullPath($Path)
    return $full.Equals($root, [StringComparison]::OrdinalIgnoreCase) -or $full.StartsWith(
        $root + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)
}

function Read-PersonalIni {
    param([string]$Path)
    $values = @{}
    if (-not [IO.File]::Exists($Path)) { return $values }
    foreach ($line in [IO.File]::ReadAllLines($Path)) {
        if ($line -match '^\s*([^#;\[=][^=]*)\s*=\s*(.*?)\s*$') {
            $values[$matches[1].Trim().ToLowerInvariant()] = $matches[2].Trim().Trim('"')
        }
    }
    return $values
}

function Test-PersonalBridgeFolder {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not [IO.Path]::IsPathRooted($Path)) { return $false }
    foreach ($name in @('Setup.exe', 'HostAgent.exe', 'PeDePeAdapter/openomsi.exe')) {
        if (-not [IO.File]::Exists((Join-Path -Path $Path -ChildPath $name))) { return $false }
    }
    return $true
}

function Find-PersonalBridgeFolder {
    param([string]$SourceRoot, [string]$DataRoot)
    $candidates = @{}
    function Add-Candidate {
        param([string]$Path, [int]$Priority)
        if (-not (Test-PersonalBridgeFolder -Path $Path)) { return }
        $full = [IO.Path]::GetFullPath($Path)
        if ((Test-PathInside -Path $full -Directory $SourceRoot) -and
            -not [IO.File]::Exists((Join-Path -Path $full -ChildPath 'app/bridge.ini'))) { return }
        if (-not $candidates.ContainsKey($full) -or $candidates[$full] -lt $Priority) {
            $candidates[$full] = $Priority
        }
    }
    function Add-Ancestors {
        param([string]$File, [int]$Priority)
        if ([string]::IsNullOrWhiteSpace($File) -or -not [IO.Path]::IsPathRooted($File)) { return }
        $parent = [IO.Path]::GetDirectoryName($File)
        for ($i = 0; $i -lt 6 -and -not [string]::IsNullOrWhiteSpace($parent); $i++) {
            Add-Candidate -Path $parent -Priority $Priority
            $parent = [IO.Path]::GetDirectoryName($parent)
        }
    }
    $recentReceipt = Join-Path $DataRoot 'Personal-Host-2.2.0.2/installed.json'
    if ([IO.File]::Exists($recentReceipt)) {
        try { Add-Candidate -Path ((ConvertFrom-Json ([IO.File]::ReadAllText($recentReceipt))).installation) -Priority 110 } catch { }
    }
    $receipt = Join-Path -Path $DataRoot -ChildPath 'Personal-Dynamic-2.2/installed.json'
    if ([IO.File]::Exists($receipt)) {
        try { Add-Candidate -Path ((ConvertFrom-Json ([IO.File]::ReadAllText($receipt))).installation) -Priority 100 }
        catch { }
    }
    if (-not [string]::IsNullOrWhiteSpace($env:APPDATA)) {
        $startup = Join-Path -Path $env:APPDATA -ChildPath 'Microsoft/Windows/Start Menu/Programs/Startup/OpenOmsi-BBS-Host.cmd'
        if ([IO.File]::Exists($startup)) {
            $text = [IO.File]::ReadAllText($startup)
            if ($text -match '"([^"\r\n]*HostAgent\.exe)"') {
                Add-Ancestors -File ($matches[1].Replace('%%', '%')) -Priority 90
            }
        }
    }
    if ($env:OS -eq 'Windows_NT') {
        foreach ($view in @([Microsoft.Win32.RegistryView]::Registry64, [Microsoft.Win32.RegistryView]::Registry32)) {
            $baseKey = $null; $key = $null
            try {
                $baseKey = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine, $view)
                $key = $baseKey.OpenSubKey('SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\Omsi.exe\OpenOMSI_BCS_Bridge')
                if ($null -ne $key) {
                    $debugger = [string]$key.GetValue('Debugger', '')
                    Add-Ancestors -File ($debugger.Trim().Trim('"')) -Priority 85
                }
            } catch { } finally {
                if ($null -ne $key) { $key.Dispose() }
                if ($null -ne $baseKey) { $baseKey.Dispose() }
            }
        }
    }
    $saved = Read-PersonalIni -Path (Join-Path -Path $DataRoot -ChildPath 'bridge.ini')
    if ($saved.ContainsKey('openomsi')) { Add-Ancestors -File $saved['openomsi'] -Priority 80 }
    $backups = Join-Path -Path $DataRoot -ChildPath 'Personal-Dynamic-2.2/Backups'
    if ([IO.Directory]::Exists($backups)) {
        foreach ($folder in @(Get-ChildItem -LiteralPath $backups -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 12)) {
            $old = Join-Path -Path $folder.FullName -ChildPath 'HostAgent.local.json'
            if ([IO.File]::Exists($old)) {
                try { Add-Ancestors -File ((ConvertFrom-Json ([IO.File]::ReadAllText($old))).server) -Priority 70 }
                catch { }
            }
        }
    }
    $current = Join-Path -Path $DataRoot -ChildPath 'HostAgent.local.json'
    if ([IO.File]::Exists($current)) {
        try {
            $hostSettings = ConvertFrom-Json ([IO.File]::ReadAllText($current))
            # An extracted personal bundle has no local bridge.ini. Prefer the saved installation.
            $parent = [IO.Path]::GetDirectoryName([string]$hostSettings.server)
            for ($i = 0; $i -lt 5 -and -not [string]::IsNullOrWhiteSpace($parent); $i++) {
                if ([IO.File]::Exists((Join-Path -Path $parent -ChildPath 'app/bridge.ini'))) {
                    Add-Candidate -Path $parent -Priority 60
                }
                $parent = [IO.Path]::GetDirectoryName($parent)
            }
        } catch { }
    }
    if ([IO.File]::Exists((Join-Path -Path $SourceRoot -ChildPath 'app/bridge.ini'))) {
        Add-Candidate -Path $SourceRoot -Priority 55
    }
    $ranked = @($candidates.GetEnumerator() | Sort-Object Value -Descending)
    if ($ranked.Count -eq 1 -or ($ranked.Count -gt 1 -and $ranked[0].Value -gt $ranked[1].Value)) {
        return [string]$ranked[0].Key
    }
    return ''
}

function Select-PersonalBridgeFolder {
    Add-Type -AssemblyName System.Windows.Forms
    $dialog = New-Object Windows.Forms.FolderBrowserDialog
    $dialog.Description = 'Selecione a pasta da sua Bridge, onde ficam Setup.exe e HostAgent.exe.'
    $dialog.ShowNewFolderButton = $false
    try {
        if ($dialog.ShowDialog() -ne [Windows.Forms.DialogResult]::OK) { throw 'Instalacao cancelada. Nenhum arquivo foi alterado.' }
        return $dialog.SelectedPath
    } finally { $dialog.Dispose() }
}

function Get-ByteHash {
    param([byte[]]$Bytes)
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return [BitConverter]::ToString($sha.ComputeHash($Bytes)).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Add-InstallFile {
    param([System.Collections.IDictionary]$Plan, [string]$Source, [string]$Target, [string]$Hash, [byte[]]$Bytes)
    $path = [IO.Path]::GetFullPath($Target)
    $key = $path.ToUpperInvariant()
    if ($Plan.Contains($key)) {
        if ($Plan[$key].Hash -ne $Hash) { throw ('Dois componentes tentam usar o mesmo arquivo: ' + $path) }
        return
    }
    $Plan.Add($key, [PSCustomObject]@{
        Path = $path; Source = $Source; Hash = $Hash; Bytes = $Bytes
        BeforeHash = ''; Existed = $false; Staged = ''; Backup = ''; Applied = $false
    })
}

function Commit-PersonalFile {
    param($Job)
    if ($Job.Existed) { [IO.File]::Replace($Job.Staged, $Job.Path, [NullString]::Value) }
    else { [IO.File]::Move($Job.Staged, $Job.Path) }
}

function Invoke-PersonalTransaction {
    param([System.Collections.IDictionary]$Plan, [string]$BackupRoot)
    $id = (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0, 8)
    $backupDirectory = Join-Path -Path $BackupRoot -ChildPath $id
    $changed = @()
    try {
        foreach ($job in $Plan.Values) {
            $job.Existed = [IO.File]::Exists($job.Path)
            if ($job.Existed) {
                $job.BeforeHash = (Get-FileHash -LiteralPath $job.Path -Algorithm SHA256).Hash.ToLowerInvariant()
                if ($job.BeforeHash -eq $job.Hash) { continue }
            }
            $null = [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($job.Path))
            $job.Staged = $job.Path + '.bbs-install-' + [Guid]::NewGuid().ToString('N') + '.tmp'
            # Register cleanup before copying, including a partially written temporary file.
            $changed += $job
            if ($null -ne $job.Bytes) { [IO.File]::WriteAllBytes($job.Staged, $job.Bytes) }
            else { [IO.File]::Copy($job.Source, $job.Staged, $false) }
            if ((Get-FileHash -LiteralPath $job.Staged -Algorithm SHA256).Hash -ne $job.Hash) {
                throw ('Arquivo incompleto no pacote: ' + [IO.Path]::GetFileName($job.Path))
            }
        }
        if ($changed.Count -eq 0) { return '' }
        $null = [IO.Directory]::CreateDirectory($backupDirectory)
        $records = @(); $index = 0
        foreach ($job in $changed) {
            if ($job.Existed) {
                if ((Get-FileHash -LiteralPath $job.Path -Algorithm SHA256).Hash -ne $job.BeforeHash) {
                    throw 'Um arquivo mudou durante a instalacao. Feche os programas da Bridge e tente novamente.'
                }
                $job.Backup = Join-Path -Path $backupDirectory -ChildPath ($index.ToString('D4') + '.bak')
                [IO.File]::Copy($job.Path, $job.Backup, $false)
                if ((Get-FileHash -LiteralPath $job.Backup -Algorithm SHA256).Hash -ne $job.BeforeHash) { throw 'Falha ao guardar a copia anterior.' }
            }
            $records += [PSCustomObject]@{ target = $job.Path; backup = $job.Backup; existed = $job.Existed }
            $index++
        }
        [IO.File]::WriteAllText((Join-Path -Path $backupDirectory -ChildPath 'backup.json'),
            (ConvertTo-Json -InputObject $records -Depth 8), (New-Object Text.UTF8Encoding($false)))
        foreach ($job in $changed) {
            if ($job.Existed) {
                if (-not [IO.File]::Exists($job.Path) -or (Get-FileHash -LiteralPath $job.Path -Algorithm SHA256).Hash -ne $job.BeforeHash) {
                    throw 'Um arquivo mudou durante a instalacao. A operacao sera revertida.'
                }
            } elseif ([IO.File]::Exists($job.Path)) { throw 'Um arquivo foi criado durante a instalacao. A operacao sera revertida.' }
            Commit-PersonalFile -Job $job
            $job.Applied = $true
        }
        return $backupDirectory
    } catch {
        $failure = $_.Exception.Message
        [array]::Reverse($changed)
        foreach ($job in $changed) {
            if (-not $job.Applied) { continue }
            try {
                if ($job.Existed) {
                    $restore = $job.Path + '.bbs-restore-' + [Guid]::NewGuid().ToString('N') + '.tmp'
                    [IO.File]::Copy($job.Backup, $restore, $false)
                    if ([IO.File]::Exists($job.Path)) { [IO.File]::Replace($restore, $job.Path, [NullString]::Value) }
                    else { [IO.File]::Move($restore, $job.Path) }
                } elseif ([IO.File]::Exists($job.Path)) { [IO.File]::Delete($job.Path) }
            } catch { Write-Host ('Restauracao manual necessaria: ' + $job.Path) -ForegroundColor Yellow }
        }
        throw ($failure + ' Copias anteriores: ' + $backupDirectory)
    } finally {
        foreach ($job in $changed) {
            if ($job.Staged -and [IO.File]::Exists($job.Staged)) { [IO.File]::Delete($job.Staged) }
        }
    }
}

function Read-HostObject {
    param([string]$Path)
    if (-not [IO.File]::Exists($Path)) { return $null }
    try {
        $text = [IO.File]::ReadAllText($Path)
        if (-not $text.TrimStart().StartsWith('{')) { throw 'object required' }
        $value = ConvertFrom-Json -InputObject $text
        if ($value -isnot [PSCustomObject]) { throw 'object required' }
        return $value
    } catch { throw ('Configuracao invalida: ' + $Path + '. Nada foi alterado.') }
}

function Get-Field {
    param($Document, [string]$Name, $Default)
    if ($null -eq $Document) { return $Default }
    $p = $Document.PSObject.Properties[$Name]
    if ($null -eq $p) { return $Default }
    return $p.Value
}

function Get-HostSelection {
    param($Document, [string]$Kind)
    $server = [string](Get-Field $Document 'server' '')
    $protocol = 0
    if ($Kind -eq 'HostAgent') {
        $company = Get-Field $Document 'company' $null
        $protocol = [int](Get-Field $company 'protocol' 0)
    } else {
        $profile = [string](Get-Field $Document 'profile' '')
        if ([IO.File]::Exists($profile)) {
            $profileObject = Read-HostObject $profile
            $protocol = [int](Get-Field $profileObject 'protocol' 0)
        }
    }
    $knownDynamic = $false
    $serverExists = [IO.File]::Exists($server)
    if ($serverExists) {
        $knownDynamic = (Get-FileHash -LiteralPath $server -Algorithm SHA256).Hash -eq '8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240'
    }
    # A missing managed sidecar is a moved/deleted installation, not a deliberate
    # custom executable. Do not infer identity from its old drive or bundle name.
    $missingManagedDynamic = $protocol -in @(0,14) -and -not $serverExists -and [IO.Path]::IsPathRooted($server) -and
        $server.Replace('\','/') -match '(?i)/app/server-dinamico-p14/openomsi\.exe$'
    $custom = Get-Field $Document 'use_custom_server' $false
    # A deliberate custom server, protocols 6/7/15 and unknown profiles stay selected.
    # The existing dynamic P14 host can move in place without changing its binary.
    $activate = $protocol -ne 15 -and ($knownDynamic -or $missingManagedDynamic -or ($Kind -eq 'HostAgent' -and $protocol -eq 14 -and -not $custom))
    return [PSCustomObject]@{ Activate = $activate; Protocol = $protocol; Server = $server; MissingManagedDynamic = $missingManagedDynamic }
}

function Restore-P15MapPreferences {
    param($Document, [string]$Path, [string]$DataRoot)
    $company = Get-Field $Document 'company' $null
    $companyID = [string](Get-Field $company 'company_id' '')
    $maps = Get-Field $Document 'maps' $null
    if (-not $companyID -or $null -eq $maps) { return $false }
    $directories = @()
    foreach ($relative in @('Personal-Host-2.2.0.2/Backups','Personal-Dynamic-2.2/Backups','Personal-Dynamic-2.2/Install-Backups')) {
        $root = Join-Path $DataRoot $relative
        if ([IO.Directory]::Exists($root)) { $directories += @(Get-ChildItem -LiteralPath $root -Directory -ErrorAction SilentlyContinue) }
    }
    $changed = $false
    foreach ($directory in @($directories | Sort-Object Name -Descending | Select-Object -First 50)) {
        try {
            $index = Join-Path $directory.FullName 'backup.json'
            if (-not [IO.File]::Exists($index)) { continue }
            $records = @(ConvertFrom-Json ([IO.File]::ReadAllText($index)))
            foreach ($record in $records) {
                if (-not $record.existed -or $record.target -ne $Path -or -not [IO.File]::Exists($record.backup)) { continue }
                if (-not (Test-PathInside $record.backup $directory.FullName)) { continue }
                $previous = Read-HostObject $record.backup
                if ([string](Get-Field (Get-Field $previous 'company' $null) 'company_id' '') -ne $companyID) { continue }
                $oldMaps = Get-Field $previous 'maps' $null
                if ($null -eq $oldMaps) { continue }
                foreach ($map in $maps.PSObject.Properties) {
                    $oldMap = Get-Field $oldMaps $map.Name $null
                    if ($null -eq $oldMap) { continue }
                    foreach ($name in @('voice','voice_radio','voice_range','public_server','require_hardware_id','admin_password')) {
                        $saved = $oldMap.PSObject.Properties[$name]
                        if ($null -eq $map.Value.PSObject.Properties[$name] -and $null -ne $saved) {
                            Set-JsonField $map.Value $name $saved.Value
                            $changed = $true
                        }
                    }
                }
            }
        } catch { continue } # A malformed old backup must not replace valid current settings.
    }
    return $changed
}

function Add-HostSettings {
    param($Plan, [string]$Path, [string]$Kind, [string]$Server, [string]$DataRoot, [bool]$RecoverP15=$false)
    $document = Read-HostObject $Path
    if ($null -eq $document) { return $false }
    if ($Kind -eq 'HostAgent' -and (Get-Field $document 'schema' 0) -ne 1) { throw 'HostAgent.local.json nao reconhecido.' }
    $custom = Get-Field $document 'use_custom_server' $false
    if ($Kind -eq 'HostAgent' -and $custom -isnot [bool]) { throw 'Opcao use_custom_server invalida.' }
    $changed = $false
    if ($Kind -eq 'HostAgent' -and $RecoverP15) { $changed = Restore-P15MapPreferences $document $Path $DataRoot }
    $selection = Get-HostSelection $document $Kind
    if ($selection.Activate -and ($selection.Server -ne $Server -or ($Kind -eq 'HostAgent' -and -not $custom))) {
        Set-JsonField $document 'server' $Server
        if ($Kind -eq 'HostAgent') { Set-JsonField $document 'use_custom_server' $true }
        $changed = $true
    }
    if (-not $changed) { return $selection.Activate }
    # Company, map options, passwords, time zone and all unknown fields stay intact.
    $bytes = (New-Object Text.UTF8Encoding($false)).GetBytes((ConvertTo-Json -InputObject $document -Depth 100) + [Environment]::NewLine)
    Add-InstallFile $Plan '' $Path (Get-ByteHash $bytes) $bytes
    return $selection.Activate
}

function Add-HostIntegrityEdit {
    param($Plan, [string]$Path, $Hashes)
    if (-not [IO.File]::Exists($Path)) { return }
    $old = [IO.File]::ReadAllText($Path)
    $updated = [regex]::Replace($old, '(?m)^([a-fA-F0-9]{64})([ \t]+)([^\r\n]+)', {
        param($m)
        $name = $m.Groups[3].Value.Replace('\', '/').TrimStart('*')
        if ($Hashes.ContainsKey($name)) { return $Hashes[$name] + $m.Groups[2].Value + $m.Groups[3].Value }
        return $m.Value
    }.GetNewClosure())
    if ($updated -ne $old) {
        $bytes = (New-Object Text.UTF8Encoding($false)).GetBytes($updated)
        Add-InstallFile $Plan '' $Path (Get-ByteHash $bytes) $bytes
    }
}

function Add-HostStartupEdit {
    param($Plan, [string]$Path, [string]$Agent, [string]$Config)
    $quotedAgent = '"' + $Agent.Replace('%','%%') + '"'
    if (-not [IO.File]::Exists($Path)) {
        $quotedConfig = '"' + $Config.Replace('%','%%') + '"'
        $text = '@echo off' + [Environment]::NewLine + 'start "" /b ' + $quotedAgent + ' --background --config ' + $quotedConfig + [Environment]::NewLine
    } else {
        $old = [IO.File]::ReadAllText($Path)
        $pattern = '(?im)^(?<prefix>[ \t]*start[ \t]+""[ \t]+(?:/b[ \t]+)?)"(?<exe>[^"\r\n]+[\\/]HostAgent\.exe)"(?<tail>[^\r\n]*)'
        $text = [regex]::Replace($old, $pattern, {
            param($m)
            if ($m.Groups['tail'].Value -notmatch '(?i)--background\b' -or $m.Groups['tail'].Value -notmatch '(?i)--config\b') { return $m.Value }
            return $m.Groups['prefix'].Value + $quotedAgent + $m.Groups['tail'].Value
        }.GetNewClosure())
        if ($text -eq $old) { return }
    }
    # Keep --config and every other argument; update only the managed agent path.
    $bytes = (New-Object Text.UTF8Encoding($false)).GetBytes($text)
    Add-InstallFile $Plan '' $Path (Get-ByteHash $bytes) $bytes
}

function Invoke-HostOnlyInstaller {
    param([string]$Destination, [string]$PreferredDestination, [switch]$NonInteractive, [switch]$NoLaunch)
    $sourceRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
    $dataRoot = Get-PersonalDataRoot $sourceRoot
    if ([string]::IsNullOrWhiteSpace($Destination) -and (Test-PersonalBridgeFolder $PreferredDestination)) { $Destination = $PreferredDestination }
    if ([string]::IsNullOrWhiteSpace($Destination)) {
        $Destination = Find-PersonalBridgeFolder $sourceRoot $dataRoot
        if ([string]::IsNullOrWhiteSpace($Destination)) {
            if ($NonInteractive) { throw 'Nao foi possivel identificar uma unica instalacao existente.' }
            $Destination = Select-PersonalBridgeFolder
        }
    }
    $destinationRoot = [IO.Path]::GetFullPath($Destination)
    if (-not (Test-PersonalBridgeFolder $destinationRoot)) { throw 'Selecione a pasta da Bridge que voce ja usa, com Setup.exe e HostAgent.exe.' }
    $busy = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
        $_.ProcessName -in @('HostAgent', 'CompanyHost', 'Setup') -or $_.ProcessName -like 'OpenOmsi_BBS_Dedicated_*'
    })
    if ($busy.Count) { throw 'Feche o HostAgent (inclusive na bandeja), CompanyHost, Setup e servidores dedicados antes de atualizar.' }
    # Protect directly launched native dedicated servers too.
    if ($env:OS -eq 'Windows_NT') {
        $native = @(Get-Process -Name openomsi -ErrorAction SilentlyContinue | Where-Object {
            try { Test-PathInside $_.Path $destinationRoot } catch { $false }
        })
        if ($native.Count) { throw 'Feche o OpenOMSI/servidor desta instalacao antes de atualizar o host.' }
    }
    $manifestPath = Join-Path $sourceRoot 'docs/HOST-PESSOAL-2.2.0.2.json'
    $manifest = Read-HostObject $manifestPath
    if ($null -eq $manifest -or $manifest.schema -ne 1 -or $manifest.version -ne '2.2.0.2' -or $manifest.edition -ne 'host-pessoal.3') { throw 'Extraia o ZIP inteiro; pacote do host nao reconhecido.' }
    $allowed = @('HostAgent.exe','CompanyHost.exe','app/server-dinamico-p14/openomsi.exe','app/server-dinamico-p14/steam_api64.dll','app/server-dinamico-p14/msvcp140.dll','app/server-dinamico-p14/vcruntime140.dll','app/server-dinamico-p14/vcruntime140_1.dll','app/server-dinamico-p14/.openomsi-content','app/server-dinamico-p14/OpenOMSI-LICENSE.txt','app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json')
    if (@($manifest.files).Count -ne $allowed.Count) { throw 'Lista de componentes do host inesperada.' }
    $plan = [ordered]@{}; $hashes = @{}; $seen = @{}
    foreach ($file in $manifest.files) {
        $relative = [string]$file.path
        if ($relative -cnotin $allowed -or $seen.ContainsKey($relative) -or $file.sha256 -notmatch '^[a-f0-9]{64}$') { throw 'Componente nao permitido neste pacote do host.' }
        $seen[$relative] = $true
        $source = Join-Path $sourceRoot $relative
        $target = Join-Path $destinationRoot $relative
        if (-not [IO.File]::Exists($source) -or -not (Test-PathInside $target $destinationRoot)) { throw 'Componente ausente; extraia o ZIP inteiro.' }
        if ((Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ne $file.sha256) { throw ('Componente corrompido: ' + $relative + '. Nada foi alterado.') }
        if ($relative -in @('HostAgent.exe','CompanyHost.exe') -and [IO.File]::Exists($target)) {
            $currentHash = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
            $known = @($manifest.previous_hosts.PSObject.Properties[$relative].Value) + @([string]$file.sha256)
            if ($currentHash -notin $known) { throw ('Host mais novo ou diferente do pacote testado: ' + $relative + '. Todos os seus arquivos foram mantidos.') }
        }
        if ($relative -eq 'app/server-dinamico-p14/openomsi.exe' -and [IO.File]::Exists($target) -and (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $file.sha256) {
            throw 'Ja existe outro servidor na pasta dedicada deste patch. Nada foi substituido.'
        }
        Add-InstallFile $plan $source $target $file.sha256
        if ($relative -in @('HostAgent.exe','CompanyHost.exe')) { $hashes[$relative] = $file.sha256 }
    }
    $server = Join-Path $destinationRoot 'app/server-dinamico-p14/openomsi.exe'
    $hostPath = Join-Path $dataRoot 'HostAgent.local.json'
    $companyPath = Join-Path $dataRoot 'CompanyHost.local.json'
    if (-not [IO.File]::Exists($companyPath)) { $companyPath = Join-Path $destinationRoot 'CompanyHost.local.json' }
    $installedHostHash = (Get-FileHash -LiteralPath (Join-Path $destinationRoot 'HostAgent.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
    # Only hosts built from the obsolete source need missing preferences recovered.
    # Complete P15 builds omit false values deliberately; never recover over those.
    $recoverP15 = $installedHostHash -in @('b02fd974513b2e5f94987f459af72fe736fb80e132a0af68d51f0df7d42e368a','9611698e6318da299c32841c08a770959693dbaa495c33caa6671ff3649cadd3')
    $hostActivated = Add-HostSettings $plan $hostPath 'HostAgent' $server $dataRoot $recoverP15
    $companyActivated = Add-HostSettings $plan $companyPath 'CompanyHost' $server $dataRoot
    foreach ($name in @('docs/SHA256.txt','SHA256SUMS.txt','docs/PATCH-HOST-SHA256.txt')) {
        Add-HostIntegrityEdit $plan (Join-Path $destinationRoot $name) $hashes
    }
    $hostDocument = Read-HostObject $hostPath
    if ($null -ne $hostDocument -and (Get-Field $hostDocument 'start_with_windows' $false) -and -not [string]::IsNullOrWhiteSpace($env:APPDATA)) {
        $startupPath = Join-Path $env:APPDATA 'Microsoft/Windows/Start Menu/Programs/Startup/OpenOmsi-BBS-Host.cmd'
        Add-HostStartupEdit $plan $startupPath (Join-Path $destinationRoot 'HostAgent.exe') $hostPath
    }
    Add-InstallFile $plan $manifestPath (Join-Path $destinationRoot 'docs/HOST-PESSOAL-2.2.0.2.json') ((Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash)
    $receipt = [PSCustomObject]@{ version='2.2.0.2'; edition='host-pessoal.3'; installation=$destinationRoot; dynamic_server=$server; host_dynamic_selected=$hostActivated; company_dynamic_selected=$companyActivated; player_files_updated=$false }
    $receiptPath = Join-Path $dataRoot 'Personal-Host-2.2.0.2/installed.json'
    $bytes = (New-Object Text.UTF8Encoding($false)).GetBytes((ConvertTo-Json $receipt -Depth 8))
    Add-InstallFile $plan '' $receiptPath (Get-ByteHash $bytes) $bytes
    Write-Host ('Atualizando seu host em: ' + $destinationRoot) -ForegroundColor Cyan
    $backup = Invoke-PersonalTransaction $plan (Join-Path $dataRoot 'Personal-Host-2.2.0.2/Backups')
    if ($hostActivated -or $companyActivated) {
        if (-not [IO.File]::Exists($server) -or (Get-FileHash -LiteralPath $server -Algorithm SHA256).Hash -ne '8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240') { throw 'Nao foi possivel verificar o servidor instalado.' }
    }
    Write-Host 'HostAgent e CompanyHost 2.2.0.2 instalados. Sua Bridge e seu Coop atuais foram preservados.' -ForegroundColor Green
    Write-Host 'Recursos P15 restaurados: voz, radio, alcance, lista publica, banimento de maquina e senha admin.'
    if ($hostActivated -or $companyActivated) { Write-Host 'Servidor dinamico P14 selecionado, com o mesmo executavel da sua edicao pessoal anterior.' }
    else { Write-Host 'Seu servidor personalizado/protocolo atual foi mantido. A copia dinamica P14 fica disponivel separadamente.' }
    if ($backup) { Write-Host ('Backup dos arquivos e configuracoes alterados: ' + $backup) }
    if ($hostActivated) { Write-Host ('Caminho do servidor verificado: ' + $server) -ForegroundColor Green }
    if (-not $NoLaunch) {
        try { Start-Process -FilePath (Join-Path $destinationRoot 'HostAgent.exe') -WorkingDirectory $destinationRoot | Out-Null }
        catch { Write-Host 'Abra o HostAgent da sua pasta instalada.' }
    }
}

if ($MyInvocation.InvocationName -ne '.') {
    $mutex = New-Object Threading.Mutex($false, 'OpenOmsiBBS-PersonalHost2202-Installer')
    $locked = $false
    try {
        $locked = $mutex.WaitOne(0)
        if (-not $locked) { throw 'Ja existe uma atualizacao do host em andamento.' }
        Invoke-HostOnlyInstaller -Destination $Destination -PreferredDestination $PreferredDestination -NonInteractive:$NonInteractive -NoLaunch:$NoLaunch
        exit 0
    } catch { Write-Host ('ERRO: ' + $_.Exception.Message) -ForegroundColor Red; exit 1 }
    finally { if ($locked) { $mutex.ReleaseMutex() }; $mutex.Dispose() }
}
