param([switch]$Restaurar)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
$pick=New-Object System.Windows.Forms.OpenFileDialog
$pick.Title='Selecione o openomsi.exe do JOGO (Coop / programa). Feche o jogo antes.'
$pick.Filter='Jogo openOMSI|openomsi.exe'
if($pick.ShowDialog() -ne 'OK'){exit}
$target=$pick.FileName
if($target -match 'server-dinamico|Dedicated_'){throw 'Escolha o jogo Coop, nao o servidor dedicado.'}
$backup=$target+'.antes-beta-passageiros'
if($Restaurar){
 if(-not(Test-Path -LiteralPath $backup)){throw 'Backup desta beta nao encontrado ao lado do jogo escolhido.'}
 Copy-Item -LiteralPath $backup -Destination $target -Force
 [System.Windows.Forms.MessageBox]::Show('Jogo anterior restaurado.','Beta de passageiros')|Out-Null
 exit
}
$source=Join-Path $PSScriptRoot 'programa\openomsi.exe'
if(-not(Test-Path -LiteralPath $backup)){Copy-Item -LiteralPath $target -Destination $backup}
Copy-Item -LiteralPath $source -Destination $target -Force
[System.Windows.Forms.MessageBox]::Show("Beta instalada com backup.`nDentro do jogo: Esc > Opcoes > Jogo.`nAtive Animacao natural, Guarda-chuvas e Passageiros pedem parada.`nHabilite Pedir parada neste mapa em cada mapa desejado.`nHostAgent e servidor nao foram substituidos.",'Beta de passageiros')|Out-Null
