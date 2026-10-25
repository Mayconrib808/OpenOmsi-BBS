param([switch]$Restaurar)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
try {
    $pick=New-Object System.Windows.Forms.OpenFileDialog
    $pick.Title='Selecione o openomsi.exe do JOGO Coop. Feche o jogo e o launcher antes.'
    $pick.Filter='Jogo openOMSI|openomsi.exe'
    if($pick.ShowDialog() -ne 'OK'){exit}
    $target=$pick.FileName
    $folder=Split-Path -Parent $target
    if($target -match 'server-dinamico|Dedicated_'){throw 'Selecione o jogo Coop, nao o servidor.'}
    $names=@('openomsi.exe','openomsi-launcher.exe','openomsi-launcher-app.exe')
    foreach($process in Get-Process -ErrorAction SilentlyContinue){
        if($process.Path -and (Split-Path -Parent $process.Path) -eq $folder -and $names -contains (Split-Path -Leaf $process.Path)){
            throw 'Feche o jogo e o launcher openOMSI da pasta selecionada antes de continuar.'
        }
    }
    $pointer=Join-Path $folder 'passageiros-beta2-backup.txt'
    function Restore-Files($manifest){
        if($manifest.folder -ne $folder){throw 'O backup pertence a outra pasta.'}
        foreach($item in $manifest.files){
            if($names -notcontains $item.name){throw 'Nome inesperado no backup.'}
            $dest=Join-Path $folder $item.name
            if($item.existed){
                $saved=Join-Path $manifest.backup $item.name
                if((Get-FileHash -LiteralPath $saved -Algorithm SHA256).Hash -ne $item.sha256){throw 'Backup alterado ou incompleto.'}
            }
        }
        foreach($item in $manifest.files){
            $dest=Join-Path $folder $item.name
            if($item.existed){Copy-Item -LiteralPath (Join-Path $manifest.backup $item.name) -Destination $dest -Force}
            elseif(Test-Path -LiteralPath $dest){Remove-Item -LiteralPath $dest -Force}
        }
    }
    if($Restaurar){
        if(-not(Test-Path -LiteralPath $pointer)){throw 'Backup da Beta 2 nao encontrado nessa pasta.'}
        $manifestPath=(Get-Content -LiteralPath $pointer -Raw).Trim()
        Restore-Files (Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json)
        [System.Windows.Forms.MessageBox]::Show('Jogo e launcher anteriores restaurados.','Passageiros Beta 2')|Out-Null
        exit
    }
    $build=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'BUILD.json') -Raw | ConvertFrom-Json
    foreach($name in $names){
        $source=Join-Path (Join-Path $PSScriptRoot 'programa') $name
        $expected=$build.hashes.PSObject.Properties[$name].Value
        if(-not $expected -or (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ne $expected){throw "Arquivo da beta incompleto: $name. Extraia o ZIP inteiro novamente."}
    }
    $backup=Join-Path $folder ('OpenOMSI-Passageiros-Backups\Beta2-'+(Get-Date -Format 'yyyyMMdd-HHmmss')+'-'+[Guid]::NewGuid().ToString('N').Substring(0,8))
    New-Item -ItemType Directory -Path $backup -Force|Out-Null
    $items=@(foreach($name in $names){
        $dest=Join-Path $folder $name
        $existed=Test-Path -LiteralPath $dest
        $hash=$null
        if($existed){Copy-Item -LiteralPath $dest -Destination (Join-Path $backup $name);$hash=(Get-FileHash -LiteralPath $dest -Algorithm SHA256).Hash}
        [PSCustomObject]@{name=$name;existed=$existed;sha256=$hash}
    })
    $manifest=[PSCustomObject]@{folder=$folder;backup=$backup;files=$items}
    $manifestPath=Join-Path $backup 'manifest.json'
    $manifest|ConvertTo-Json -Depth 5|Set-Content -LiteralPath $manifestPath -Encoding UTF8
    try {
        foreach($name in $names){
            $dest=Join-Path $folder $name
            Copy-Item -LiteralPath (Join-Path (Join-Path $PSScriptRoot 'programa') $name) -Destination $dest -Force
            if((Get-FileHash -LiteralPath $dest -Algorithm SHA256).Hash -ne $build.hashes.PSObject.Properties[$name].Value){throw "Copia incompleta: $name"}
        }
        Set-Content -LiteralPath $pointer -Value $manifestPath -Encoding UTF8
    } catch {Restore-Files $manifest;throw}
    [System.Windows.Forms.MessageBox]::Show("Beta 2 instalada com backup do jogo e do launcher.`nAbra o lobby > Configuracoes > Jogabilidade > Passageiros.`nSelecione Natural, ative Guarda-chuvas e Passageiros dao sinal, e escolha os mapas.`nReinicie o jogo para aplicar as escolhas.",'Passageiros Beta 2')|Out-Null
} catch {
    [System.Windows.Forms.MessageBox]::Show($_.Exception.Message,'Instalacao interrompida')|Out-Null
    exit 1
}
