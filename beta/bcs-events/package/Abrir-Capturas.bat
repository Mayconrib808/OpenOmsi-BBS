@echo off
setlocal
if not exist "%LOCALAPPDATA%\OpenOmsi-BBS\EventosBeta" mkdir "%LOCALAPPDATA%\OpenOmsi-BBS\EventosBeta"
start "" "%LOCALAPPDATA%\OpenOmsi-BBS\EventosBeta"
endlocal
