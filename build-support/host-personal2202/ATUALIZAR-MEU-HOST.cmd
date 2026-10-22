@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\atualizar_host_pessoal.ps1" %*
set "BBS_HOST_RESULT=%ERRORLEVEL%"
echo.
pause
exit /b %BBS_HOST_RESULT%
