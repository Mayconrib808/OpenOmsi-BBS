@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\atualizar_host_pessoal.ps1" -PreferredDestination "D:\Download\OpenOmsi_BBS_2.1.1 Host" %*
set "BBS_HOST_RESULT=%ERRORLEVEL%"
echo.
pause
exit /b %BBS_HOST_RESULT%
