@echo off
setlocal
chcp 65001 >nul
for %%I in ("%~dp0..") do set "BCS_BETA_ROOT=%%~fI"
if not exist "%BCS_BETA_ROOT%\launcher.exe" (
 echo Extraia a pasta Beta-Eventos-BCS dentro de OMSI 2\Busbetrieb-Simulator.
 pause
 exit /b 1
)
if not exist "%~dp0bcs-event-probe.jar" (
 echo O arquivo bcs-event-probe.jar esta faltando. Extraia o ZIP completo.
 pause
 exit /b 1
)
echo Feche o BCS antes de iniciar este beta.
echo Capturas: %LOCALAPPDATA%\OpenOmsi-BBS\EventosBeta
echo Este beta registra eventos; ainda nao cria cenas 3D.
echo.
set "JAVA_TOOL_OPTIONS=%JAVA_TOOL_OPTIONS% -javaagent:"%~dp0bcs-event-probe.jar"="%LOCALAPPDATA%\OpenOmsi-BBS\EventosBeta""
start "" /D "%BCS_BETA_ROOT%" "%BCS_BETA_ROOT%\launcher.exe"
endlocal
