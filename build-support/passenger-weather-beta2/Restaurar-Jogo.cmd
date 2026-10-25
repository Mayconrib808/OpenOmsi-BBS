@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Instalar-Beta.ps1" -Restaurar
if errorlevel 1 pause
