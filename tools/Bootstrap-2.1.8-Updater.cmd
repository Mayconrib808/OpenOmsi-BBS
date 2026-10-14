@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul 2>&1
title OpenOMSI BCS Bridge 2.1.8 - Updater bootstrap

rem Emergency bootstrap for the already approved 2.1.8 updater.
rem It intentionally launches the updater WITHOUT --wait-setup so installations
rem affected by the broken PID/path handoff can complete the approved update.
rem The exact installer SHA-256 and byte size come from the signed 2.1.8 manifest.
set "EXPECTED=c760c4c6563421805c3cf9ee0cf6d9a70e0bea719492dc4dc10115ee05e287cd"
set "EXPECTED_SIZE=21773312"
set "UPDATES=%LOCALAPPDATA%\OpenOmsi-BBS\Updates"
set "UPDATER=%UPDATES%\Atualizar-Bridge-%EXPECTED%.exe"
set "URL=https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/%EXPECTED%.exe"

for /f "usebackq delims=" %%L in (`powershell -NoProfile -Command "[Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName" 2^>nul`) do set "UILANG=%%L"
if /I "%UILANG%"=="de" goto :MSG_DE
if /I "%UILANG%"=="en" goto :MSG_EN
goto :MSG_PT

:MSG_PT
echo ================================================================
echo OpenOMSI BCS Bridge 2.1.8 - Bootstrap do atualizador
echo ================================================================
echo.
echo Este bootstrap contorna SOMENTE o handoff quebrado do Setup 2.1.8.
echo Ele usa o atualizador oficial ja aprovado, verifica tamanho e SHA-256
echo e o abre SEM --wait-setup, que causa o erro de processo/Setup.
echo.
echo Feche antes: Setup, BCS/BBS, OMSI, openOMSI, HostAgent e CompanyHost.
echo.
set "T_DOWN=Baixando o atualizador 2.1.8 aprovado..."
set "T_VERIFY=Verificando tamanho e SHA-256..."
set "T_BAD=ERRO: o arquivo nao corresponde ao atualizador 2.1.8 aprovado."
set "T_RUN=Iniciando o atualizador sem o handoff quebrado..."
set "T_FAIL=O atualizador retornou um erro. Leia a mensagem exibida por ele."
set "T_DONE=Bootstrap concluido. Se a Bridge mostrou 'Bridge atualizada', terminou."
goto :START

:MSG_EN
echo ================================================================
echo OpenOMSI BCS Bridge 2.1.8 - Updater bootstrap
echo ================================================================
echo.
echo This bootstrap only bypasses the broken Setup 2.1.8 handoff.
echo It uses the already approved updater, verifies size and SHA-256,
echo and launches it WITHOUT --wait-setup, which causes the Setup/process error.
echo.
echo Close first: Setup, BCS/BBS, OMSI, openOMSI, HostAgent and CompanyHost.
echo.
set "T_DOWN=Downloading the approved 2.1.8 updater..."
set "T_VERIFY=Verifying size and SHA-256..."
set "T_BAD=ERROR: the file does not match the approved 2.1.8 updater."
set "T_RUN=Starting the updater without the broken handoff..."
set "T_FAIL=The updater returned an error. Read the message displayed by it."
set "T_DONE=Bootstrap finished. If Bridge displayed 'Bridge updated', it is complete."
goto :START

:MSG_DE
echo ================================================================
echo OpenOMSI BCS Bridge 2.1.8 - Updater-Bootstrap
echo ================================================================
echo.
echo Dieser Bootstrap umgeht nur die fehlerhafte Setup-2.1.8-Uebergabe.
echo Er nutzt den bereits freigegebenen Updater, prueft Groesse und SHA-256
echo und startet ihn OHNE --wait-setup, das den Setup/Prozess-Fehler verursacht.
echo.
echo Vorher schliessen: Setup, BCS/BBS, OMSI, openOMSI, HostAgent und CompanyHost.
echo.
set "T_DOWN=Freigegebenen 2.1.8-Updater herunterladen..."
set "T_VERIFY=Groesse und SHA-256 werden geprueft..."
set "T_BAD=FEHLER: Die Datei entspricht nicht dem freigegebenen 2.1.8-Updater."
set "T_RUN=Updater ohne die fehlerhafte Uebergabe starten..."
set "T_FAIL=Der Updater hat einen Fehler gemeldet. Bitte die angezeigte Meldung lesen."
set "T_DONE=Bootstrap abgeschlossen. Wenn 'Bridge aktualisiert' angezeigt wurde, ist alles fertig."
goto :START

:START
if not defined LOCALAPPDATA (
  echo ERROR: LOCALAPPDATA is not available.
  pause
  exit /b 10
)
if not exist "%UPDATES%" mkdir "%UPDATES%" >nul 2>&1
if errorlevel 1 (
  echo ERROR: Could not create "%UPDATES%".
  pause
  exit /b 11
)

call :VERIFY
if "%VALID%"=="1" goto :RUN
if exist "%UPDATER%" del /q "%UPDATER%" >nul 2>&1

echo %T_DOWN%
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ProgressPreference='SilentlyContinue'; Invoke-WebRequest -UseBasicParsing -Uri $env:URL -OutFile $env:UPDATER"
if errorlevel 1 (
  echo ERROR: download failed.
  pause
  exit /b 12
)
call :VERIFY
if not "%VALID%"=="1" (
  echo %T_BAD%
  if exist "%UPDATER%" del /q "%UPDATER%" >nul 2>&1
  pause
  exit /b 13
)

goto :RUN

:VERIFY
set "VALID=0"
if not exist "%UPDATER%" exit /b 0
for %%F in ("%UPDATER%") do set "ACTUAL_SIZE=%%~zF"
if not "%ACTUAL_SIZE%"=="%EXPECTED_SIZE%" exit /b 0
set "ACTUAL="
for /f "usebackq delims=" %%H in (`powershell -NoProfile -Command "(Get-FileHash -Algorithm SHA256 -LiteralPath $env:UPDATER).Hash.ToLowerInvariant()" 2^>nul`) do set "ACTUAL=%%H"
if /I "%ACTUAL%"=="%EXPECTED%" set "VALID=1"
exit /b 0

:RUN
echo %T_VERIFY%
echo.
echo %T_RUN%
echo.
"%UPDATER%"
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
  echo.
  echo %T_FAIL%
  echo Exit code: %RC%
  pause
  exit /b %RC%
)
echo.
echo %T_DONE%
pause
exit /b 0
