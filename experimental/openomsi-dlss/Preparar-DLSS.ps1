param([string]$StreamlineSource = '')
$ErrorActionPreference = 'Stop'
$game = $PSScriptRoot
$pt = (Get-Culture).Name -match '^pt'
$de = (Get-Culture).Name -match '^de'
function Say([string]$p, [string]$e, [string]$g) {
    if ($pt) { Write-Host $p } elseif ($de) { Write-Host $g } else { Write-Host $e }
}

try {
    if (Get-Process -Name 'openomsi', 'openomsi-launcher', 'openomsi-launcher-app' -ErrorAction SilentlyContinue) {
        throw 'Close openOMSI and its launcher / Feche o openOMSI e o launcher / openOMSI und Launcher schliessen.'
    }
    if (!(Test-Path -LiteralPath (Join-Path $game 'openomsi.exe'))) {
        throw 'Run this script beside openomsi.exe / Execute junto de openomsi.exe.'
    }
    $names = @('sl.interposer.dll','sl.common.dll','sl.dlss.dll','nvngx_dlss.dll')
    $missing = @($names | Where-Object { !(Test-Path -LiteralPath (Join-Path $game $_)) })
    if ($missing.Count) {
        if (!$StreamlineSource) {
            $old = Join-Path $env:USERPROFILE 'Downloads\openOMSI-0.2.314-windows-x64'
            if (Test-Path -LiteralPath $old) {
                $ready = @($names | Where-Object { Test-Path -LiteralPath (Join-Path $old $_) })
                if ($ready.Count -eq 4) { $StreamlineSource = $old }
            }
        }
        if (!$StreamlineSource) {
            Say 'Baixando o SDK Streamline 2.14.1 da NVIDIA...' 'Downloading NVIDIA Streamline SDK 2.14.1...' 'NVIDIA Streamline SDK 2.14.1 wird heruntergeladen...'
            $cache = Join-Path $env:LOCALAPPDATA 'OpenOMSI-DLSS-Test\Streamline-2.14.1'
            New-Item -ItemType Directory -Path $cache -Force | Out-Null
            $zip = Join-Path $cache 'streamline-sdk.zip'
            if (!(Test-Path -LiteralPath $zip)) {
                Invoke-WebRequest -Uri 'https://github.com/NVIDIA-RTX/Streamline/releases/download/v2.14.1/streamline-sdk-v2.14.1.zip' -OutFile $zip
            }
            $sdk = Join-Path $cache 'sdk'
            if (!(Test-Path -LiteralPath $sdk)) { Expand-Archive -LiteralPath $zip -DestinationPath $sdk }
            $candidate = Get-ChildItem -LiteralPath $sdk -Filter 'sl.interposer.dll' -File -Recurse |
                Where-Object { $_.DirectoryName -match '[\\/]bin[\\/]x64$' } | Select-Object -First 1
            if (!$candidate) { throw 'Streamline bin/x64 was not found in the official SDK.' }
            $StreamlineSource = $candidate.DirectoryName
        }
        foreach ($name in $missing) {
            $source = Join-Path $StreamlineSource $name
            if (!(Test-Path -LiteralPath $source)) { throw "Missing SDK file: $source" }
            Copy-Item -LiteralPath $source -Destination (Join-Path $game $name)
        }
    }
    Say 'DLLs do DLSS nativo preparadas.' 'Native DLSS DLLs prepared.' 'Native DLSS-DLLs vorbereitet.'

    $ini = Join-Path $game 'ReShade.ini'
    if (Test-Path -LiteralPath $ini) {
        $data = Join-Path $env:LOCALAPPDATA 'OpenOMSI-ReShade\0.2.50-dlss-test.1'
        $backup = Join-Path $game ('ReShade-backup-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
        New-Item -ItemType Directory -Path $data,$backup -Force | Out-Null
        Get-ChildItem -LiteralPath $game -Filter 'ReShade*.ini' -File | Copy-Item -Destination $backup
        $text = [IO.File]::ReadAllText($ini)
        $presetSource = Join-Path $game 'ReShadePreset.ini'
        if ($text -match '(?m)^BasePath=(.+)$') {
            $previous = $Matches[1].Trim()
            $previousIni = Join-Path $previous 'ReShade.ini'
            if (Test-Path -LiteralPath $previousIni) {
                $text = [IO.File]::ReadAllText($previousIni)
                $presetSource = Join-Path $previous 'ReShadePreset.ini'
                Get-ChildItem -LiteralPath $previous -Filter 'ReShade*.ini' -File | Copy-Item -Destination $backup -Force
            }
        }
        $preset = "Techniques=`r`nTechniqueSorting=`r`n"
        if (Test-Path -LiteralPath $presetSource) {
            $preset = [IO.File]::ReadAllText($presetSource)
            if ($preset -notmatch '(?m)^Techniques=') { $preset = "Techniques=`r`n" + $preset }
        }
        $text = $text -replace '(?m)^(BasePath|AddonPath|PresetPath|StartupPresetPath|EffectSearchPaths|TextureSearchPaths|IntermediateCachePath)=.*(?:\n|$)', ''
        $text += @"

[ADDON]
AddonPath=$game

[GENERAL]
PresetPath=$data\ReShadePreset.ini
StartupPresetPath=
EffectSearchPaths=$game\reshade-shaders\Shaders\**
TextureSearchPaths=$game\reshade-shaders\Textures\**
"@
        foreach ($file in @($ini,"$data\ReShade.ini","$data\ReShadePreset.ini")) {
            if (Test-Path -LiteralPath $file) { (Get-Item -LiteralPath $file).IsReadOnly = $false }
        }
        $utf8 = [Text.UTF8Encoding]::new($false)
        [IO.File]::WriteAllText("$data\ReShadePreset.ini", $preset, $utf8)
        [IO.File]::WriteAllText("$data\ReShade.ini", $text, $utf8)
        [IO.File]::WriteAllText($ini, "[INSTALL]`r`nBasePath=$data`r`n", $utf8)
        Say 'ReShade preparado com backup e gravação na pasta do usuário.' 'ReShade prepared with a backup and user-writable configuration.' 'ReShade mit Sicherung und beschreibbarer Benutzerkonfiguration vorbereitet.'
    } else {
        Say 'Agora instale pela rota DLSS nativo no Swapper, selecionando ESTA pasta. Depois feche o Swapper e execute este arquivo novamente.' 'Now install the Native DLSS route in Swapper, selecting THIS folder. Then exit Swapper and run this file again.' 'Jetzt in Swapper die Route Native DLSS in DIESEM Ordner installieren. Danach Swapper beenden und diese Datei erneut ausfuehren.'
    }
    Say 'No launcher: DirectX 12 e NVIDIA DLSS / DLAA. Para o Bridge, escolha o openomsi.exe desta pasta no Setup.' 'In the launcher: DirectX 12 and NVIDIA DLSS / DLAA. For Bridge, select this folder\openomsi.exe in Setup.' 'Im Launcher: DirectX 12 und NVIDIA DLSS / DLAA. In Bridge-Setup openomsi.exe aus diesem Ordner waehlen.'
} catch {
    Write-Error $_
    exit 1
}
