@echo off
setlocal
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Verificar-Captura.ps1" -OpenFolder
pause
endlocal
