#!/usr/bin/env python3
"""Build an opt-in, SHA-guarded Setup repair for the *approved* 2.2.3.1 ZIP."""
from pathlib import Path
import hashlib, shutil, json

ROOT = Path(__file__).resolve().parents[1]
source = ROOT / "activation-runtime" / "Setup.exe"
assert source.is_file() and source.stat().st_size > 1_000_000
new_hash = hashlib.sha256(source.read_bytes()).hexdigest()
assert new_hash != "147150876c33dbb27880611372365655a9973d809e9592d13846ada731d7dc0e"
out = ROOT / "repair-kit"
(out / "correcao").mkdir(parents=True, exist_ok=True)
shutil.copy2(source, out / "correcao" / "Setup.exe")
cmd = r"""@echo off
setlocal EnableExtensions
cd /d "%~dp0"
if not exist "app\OpenOMSI_BCS_Bridge.exe" goto wrongfolder
if not exist "correcao\Setup.exe" goto wrongfolder
if not exist "Setup.exe" goto wrongfolder
certutil -hashfile "Setup.exe" SHA256 2>NUL | findstr /I /C:"147150876c33dbb27880611372365655a9973d809e9592d13846ada731d7dc0e" >NUL
if errorlevel 1 goto unrecognized
if exist "Setup.exe.antes-do-reparo.bak" goto unrecognized
move "Setup.exe" "Setup.exe.antes-do-reparo.bak" >NUL
if errorlevel 1 goto failed
copy /B "correcao\Setup.exe" "Setup.exe" >NUL
if errorlevel 1 goto restore
echo REPARO APLICADO: Setup da Bridge 2.2.3.1 corrigido.
echo Agora abra Setup.exe, selecione "Usar Coop incluido", e "Salvar e ativar".
pause
exit /B 0
:restore
move "Setup.exe.antes-do-reparo.bak" "Setup.exe" >NUL
:failed
echo Nao foi possivel substituir o Setup. Feche o Setup e verifique permissao da pasta.
pause
exit /B 1
:wrongfolder
echo Extraia ESTE KIT na pasta que contem o Setup.exe ORIGINAL da Bridge 2.2.3.1.
echo Nao coloque o reparo na pasta do OMSI 2.
pause
exit /B 1
:unrecognized
echo Este Setup nao e o da publicacao 2.2.3.1 aprovada. NADA foi alterado.
echo Nao use em versoes antigas, novas ou ja corrigidas.
pause
exit /B 1
"""
(out / "APLICAR-REPARO-2231.cmd").write_bytes(cmd.replace("\r\n", "\n").replace("\n", "\r\n").encode("ascii"))
(out / "LEIA-ME-PT-BR.txt").write_text(
    "REPARO VERIFICADO - Bridge 2.2.3.1 (somente a versao original publicada)\n\n"
    "1. Feche BCS, OMSI, openOMSI e o Setup.\n"
    "2. Extraia este kit DENTRO da pasta onde esta o Setup.exe da Bridge 2.2.3.1.\n"
    "3. Execute APLICAR-REPARO-2231.cmd. O script compara SHA-256, faz backup do Setup antigo e substitui apenas Setup.exe.\n"
    "4. Abra Setup.exe, escolha 'Usar Coop incluido' e 'Salvar e ativar'.\n"
    "5. Se surgir 'Access is denied' com chave IFEO, a permissao no Registro foi recusada pelo Windows; nao execute REGEDIT nem force permissoes.\n"
    "   Configure no BCS a integracao NATIVA OpenOMSI/PeDePeAdapter desta Bridge (sem interceptacao IFEO) e use 'Salvar e ativar'.\n"
    "6. O kit NAO altera o jogo, BCS.jar, HostAgent, servidor, mapas ou configuracoes pessoais.\n\n"
    f"SHA256 novo Setup.exe: {new_hash}\n", encoding="utf-8")
(out / "LEAME-ES.txt").write_text(
    "REPARACION VERIFICADA - Bridge 2.2.3.1 (solo la version original publicada)\n\n"
    "1. Cierra BBS/BCS, OMSI, openOMSI y Setup.\n"
    "2. Extrae este kit DENTRO de la carpeta que contiene el Setup.exe de Bridge 2.2.3.1.\n"
    "3. Ejecuta APLICAR-REPARO-2231.cmd. Verifica SHA-256, guarda una copia del Setup anterior y reemplaza solo Setup.exe.\n"
    "4. Abre Setup.exe; selecciona 'Usar Coop incluido' y 'Guardar y activar'.\n"
    "5. Si aparece 'Access is denied' para la clave IFEO, Windows ha denegado permisos del Registro. No modifiques permisos ni uses REGEDIT.\n"
    "   Selecciona en BBS/BCS la integracion NATIVA de openOMSI mediante PeDePeAdapter de esta Bridge, sin la antigua interceptacion IFEO.\n"
    "6. Este kit no modifica el juego, el BBS.jar, HostAgent, los servidores, mapas ni datos personales.\n\n"
    f"SHA256 del nuevo Setup.exe: {new_hash}\n", encoding="utf-8")
(out / "provenance.json").write_text(json.dumps({
    "baseline_version":"2.2.3.1",
    "baseline_approved_run":42116052029,
    "baseline_approved_artifact":13457334107,
    "baseline_setup_sha256":"147150876c33dbb27880611372365655a9973d809e9592d13846ada731d7dc0e",
    "repaired_setup_sha256":new_hash,
    "scope":"Setup.exe only; no game binaries or registry security bypass"
},indent=2)+"\n",encoding="utf-8")
print("Setup repair kit ready; source SHA256", new_hash)
