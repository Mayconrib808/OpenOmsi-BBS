param([Parameter(Mandatory=$true)][string]$Package, [Parameter(Mandatory=$true)][string]$Baseline)
Set-StrictMode -Version 2.0
$ErrorActionPreference='Stop'
$Package=[IO.Path]::GetFullPath($Package)
. (Join-Path $Package 'scripts/atualizar_host_pessoal.ps1')
$oldLocal=$env:LOCALAPPDATA; $oldRoaming=$env:APPDATA
$sandbox=Join-Path ([IO.Path]::GetTempPath()) ('host2202-tests-'+[Guid]::NewGuid().ToString('N'))
$null=[IO.Directory]::CreateDirectory($sandbox)
$passed=New-Object 'System.Collections.Generic.List[string]'
function Assert-True { param($Value,[string]$Message) if (-not $Value) {throw $Message} }
function Put-File { param([string]$Path,[string]$Text) $null=[IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path)); [IO.File]::WriteAllText($Path,$Text) }
function Snapshot { param([string]$Root) $values=@{}; foreach($f in @(Get-ChildItem -LiteralPath $Root -Recurse -File)){ $values[$f.FullName]=(Get-FileHash -LiteralPath $f.FullName -Algorithm SHA256).Hash }; return $values }
function Assert-Snapshot { param($Before) foreach($p in $Before.Keys){ Assert-True ([IO.File]::Exists($p)) ('Removed: '+$p); Assert-True ((Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash -eq $Before[$p]) ('Changed: '+$p) } }
function Fixture {
    param([int]$Protocol=14,[bool]$Custom=$false)
    $base=Join-Path $sandbox ([Guid]::NewGuid().ToString('N'))
    $install=Join-Path $base 'Minha Bridge com espacos'
    $data=Join-Path $base 'local/OpenOmsi-BBS'
    $env:LOCALAPPDATA=Join-Path $base 'local'; $env:APPDATA=Join-Path $base 'roaming'
    $null=[IO.Directory]::CreateDirectory($install); $null=[IO.Directory]::CreateDirectory($data)
    foreach($name in @('HostAgent.exe','CompanyHost.exe')){ Copy-Item -LiteralPath (Join-Path $Baseline $name) -Destination (Join-Path $install $name) }
    foreach($name in @('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe','app/compat/Omsi.exe','app/compat/omsi-plugin-host32.exe','OpenOmsi Coop 0.3.38/programa/openomsi.exe','OpenOmsi Coop 0.3.38/source/openomsi-coop8-source.zip','app/bridge.ini','HostedMaps/meu-mapa/server.cfg','meu-arquivo-extra.txt')){ Put-File (Join-Path $install $name) ('Coop8/Bridge2.2.0.2 preservado: '+$name) }
    $server=Join-Path $install 'app/server/openomsi.exe'; Put-File $server ('current-server-protocol-'+$Protocol)
    $hostDoc=[ordered]@{schema=1;server=$server;use_custom_server=$Custom;company=@{protocol=$Protocol;company_id='transfort-br';clock=@{time_zone='Europe/Berlin';shift_minutes=-480}};maps=@{mapa=@{traffic=45;max_people=1400;metar_station='SBBH';port=27015;enabled=$true}};host_key='private-key-kept';room_id='private-room';control_key='private-control';start_with_windows=$true;unknown=@{keep='yes'}}
    Put-File (Join-Path $data 'HostAgent.local.json') (ConvertTo-Json $hostDoc -Depth 20)
    $profile=Join-Path $base 'company.json'; Put-File $profile (ConvertTo-Json @{protocol=$Protocol;company_id='transfort-br'})
    Put-File (Join-Path $data 'CompanyHost.local.json') (ConvertTo-Json @{Server=$server;Profile=$profile;Root='F:\OMSI 2';Config='F:\Original\server.cfg';Session='my-map';Share='F:\export.json';Keep=@{n=3}} -Depth 10)
    Put-File (Join-Path $install 'docs/SHA256.txt') (('a'*64)+"  HostAgent.exe`r`n"+('b'*64)+"  CompanyHost.exe`r`n"+('c'*64)+"  Setup.exe`r`n")
    return [PSCustomObject]@{Root=$install;Data=$data;Base=$base;Server=$server}
}
function Get-Process { [CmdletBinding()]param([string]$Name) return @() }
try {
    $f=Fixture
    $before=Snapshot $f.Root
    $player=@{}; foreach($p in $before.Keys){ if($p -notin @((Join-Path $f.Root 'HostAgent.exe'),(Join-Path $f.Root 'CompanyHost.exe'),(Join-Path $f.Root 'docs/SHA256.txt'))){$player[$p]=$before[$p]} }
    $beforeDoc=Read-HostObject (Join-Path $f.Data 'HostAgent.local.json')
    Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch
    Assert-Snapshot $player
    $afterDoc=Read-HostObject (Join-Path $f.Data 'HostAgent.local.json')
    Assert-True ($afterDoc.use_custom_server -and $afterDoc.server -eq (Join-Path $f.Root 'app/server-dinamico-p14/openomsi.exe')) 'P14 not selected'
    foreach($name in @('company','maps','host_key','room_id','control_key','unknown','start_with_windows')){ Assert-True ((ConvertTo-Json $beforeDoc.$name -Depth 20 -Compress) -eq (ConvertTo-Json $afterDoc.$name -Depth 20 -Compress)) ('Preference changed: '+$name) }
    $integrity=[IO.File]::ReadAllText((Join-Path $f.Root 'docs/SHA256.txt'))
    Assert-True ($integrity.Contains(('c'*64)+'  Setup.exe')) 'Player checksum changed'
    Assert-True ($integrity.Contains((Get-FileHash (Join-Path $f.Root 'HostAgent.exe')).Hash.ToLowerInvariant())) 'Host manifest not refreshed'
    $startup=Join-Path $env:APPDATA 'Microsoft/Windows/Start Menu/Programs/Startup/OpenOmsi-BBS-Host.cmd'
    Assert-True ([IO.File]::ReadAllText($startup).Contains($f.Root)) 'Startup points outside installed folder'
    $passed.Add('actual_installation_keeps_bridge_coop8_paths_company_maps_credentials_and_server_cfg')
    $snap=Snapshot $f.Root; $hostBytes=[IO.File]::ReadAllText((Join-Path $f.Data 'HostAgent.local.json'))
    Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch
    Assert-Snapshot $snap
    Assert-True ([IO.File]::ReadAllText((Join-Path $f.Data 'HostAgent.local.json')) -eq $hostBytes) 'Repeat rewrites preferences'
    $passed.Add('repeat_install_is_idempotent')
    Put-File (Join-Path $f.Data 'Personal-Host-2.2.0.2/installed.json') (ConvertTo-Json @{installation=$f.Root})
    Assert-True ((Find-PersonalBridgeFolder $Package $f.Data) -eq $f.Root) 'Saved installation not detected'
    $passed.Add('detects_existing_installation_instead_of_downloaded_pack')

    foreach($protocol in @(6,7,15)) {
        $f=Fixture $protocol $false; $host=Snapshot $f.Data; $serverBefore=(Get-FileHash $f.Server).Hash
        Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch
        Assert-Snapshot $host
        Assert-True ((Get-FileHash $f.Server).Hash -eq $serverBefore) 'Existing native server overwritten'
        $passed.Add('protocol_'+$protocol+'_configuration_and_server_preserved')
    }
    $f=Fixture 14 $true
    Copy-Item -LiteralPath (Join-Path $Baseline 'app/server/openomsi.exe') -Destination $f.Server -Force
    $oldNative=(Get-FileHash $f.Server).Hash
    Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch
    $hd=Read-HostObject (Join-Path $f.Data 'HostAgent.local.json')
    $cd=Read-HostObject (Join-Path $f.Data 'CompanyHost.local.json')
    Assert-True ($hd.server -eq $cd.Server -and $hd.server -eq (Join-Path $f.Root 'app/server-dinamico-p14/openomsi.exe')) 'Existing personal dynamic server was not retained/selected'
    Assert-True ((Get-FileHash $f.Server).Hash -eq $oldNative -and (Get-FileHash $hd.server).Hash -eq $oldNative) 'Native dynamic binary changed'
    Assert-True ($cd.Config -eq 'F:\Original\server.cfg' -and $cd.Keep.n -eq 3) 'Existing server configuration changed'
    $passed.Add('existing_personal_dynamic_p14_keeps_exact_native_and_companyhost_configuration')
    $f=Fixture 14 $true; $host=Snapshot $f.Data
    Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch
    Assert-Snapshot $host; $passed.Add('explicit_custom_p14_server_preserved')

    $f=Fixture; Put-File (Join-Path $f.Data 'HostAgent.local.json') '{bad-json'; $snap=Snapshot $f.Root
    $failed=$false; try { Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch } catch {$failed=$true}
    Assert-True $failed 'Malformed settings accepted'; Assert-Snapshot $snap; $passed.Add('malformed_settings_abort_before_any_write')
    $f=Fixture; Put-File (Join-Path $f.Root 'HostAgent.exe') 'newer or private host'; $snap=Snapshot $f.Root
    $failed=$false; try { Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch } catch {$failed=$true}
    Assert-True $failed 'Unknown/newer host overwritten'; Assert-Snapshot $snap; $passed.Add('unknown_or_newer_host_cannot_be_downgraded')

    $f=Fixture; $saved=[IO.File]::ReadAllBytes((Join-Path $Package 'HostAgent.exe'))
    try {
        [IO.File]::WriteAllText((Join-Path $Package 'HostAgent.exe'),'corrupt payload'); $snap=Snapshot $f.Root
        $failed=$false; try { Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch } catch {$failed=$true}
        Assert-True $failed 'Corrupt payload accepted'; Assert-Snapshot $snap
    } finally { [IO.File]::WriteAllBytes((Join-Path $Package 'HostAgent.exe'),$saved) }
    $passed.Add('corrupt_payload_aborts_without_writing')
    $f=Fixture; $snap=Snapshot $f.Root
    $originalCommit=${function:Commit-PersonalFile}; $script:commitCount=0
    try {
        function Commit-PersonalFile { param($Job) $script:commitCount++; if($script:commitCount -eq 2){throw 'Injected disk failure'}; if($Job.Existed){[IO.File]::Replace($Job.Staged,$Job.Path,[NullString]::Value)}else{[IO.File]::Move($Job.Staged,$Job.Path)} }
        $failed=$false; try{Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch}catch{$failed=$true}
        Assert-True $failed 'Commit failure ignored'; Assert-Snapshot $snap
        Assert-True (-not [IO.File]::Exists((Join-Path $f.Root 'app/server-dinamico-p14/openomsi.exe'))) 'Partial new server left installed'
    } finally { Set-Item Function:Commit-PersonalFile $originalCommit }
    $passed.Add('failed_commit_restores_previous_host_and_removes_partial_install')
    $f=Fixture; $snap=Snapshot $f.Root
    try {
        function Get-Process { [CmdletBinding()]param([string]$Name) [PSCustomObject]@{ProcessName='HostAgent'} }
        $failed=$false; try{Invoke-HostOnlyInstaller -Destination $f.Root -NonInteractive -NoLaunch}catch{$failed=$true}
        Assert-True $failed 'Running host accepted'; Assert-Snapshot $snap
    } finally { function Get-Process { [CmdletBinding()]param([string]$Name) return @() } }
    $passed.Add('running_host_blocks_update_without_changes')
    $result=[ordered]@{status='passed';powershell=$PSVersionTable.PSVersion.ToString();passed=@($passed.ToArray());real_two_player_bcs_trip_tested=$false}
    $out=Join-Path $Package ('validation/installer-powershell-'+$PSVersionTable.PSVersion.Major+'.json')
    $null=[IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($out)); [IO.File]::WriteAllText($out,(ConvertTo-Json $result -Depth 10))
    Write-Host ('HOST INSTALLER TESTS PASSED: '+$passed.Count+' scenarios; PowerShell '+$PSVersionTable.PSVersion)
} finally { $env:LOCALAPPDATA=$oldLocal; $env:APPDATA=$oldRoaming; if([IO.Directory]::Exists($sandbox)){Remove-Item -LiteralPath $sandbox -Recurse -Force} }
