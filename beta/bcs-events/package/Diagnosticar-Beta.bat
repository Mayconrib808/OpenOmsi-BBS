@echo off
setlocal
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Iniciar-Beta.ps1" -PreflightOnly
endlocal
