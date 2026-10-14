@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Preparar-DLSS.ps1" %*
pause
