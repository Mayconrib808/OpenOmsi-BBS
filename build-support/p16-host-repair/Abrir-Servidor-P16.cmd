@echo off
chcp 65001 >nul
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0Abrir-Servidor-P16.ps1"
if errorlevel 1 (
 echo.
 echo A correcao nao foi concluida. Confira a mensagem acima.
 pause
 exit /b 1
)
