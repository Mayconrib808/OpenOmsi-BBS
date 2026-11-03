@echo off
if not exist "%~dp0logs" mkdir "%~dp0logs"
copy /Y "%USERPROFILE%\.openomsi\game.log" "%~dp0logs\game.log"
copy /Y "%~dp0beta-manifest.json" "%~dp0logs\beta-manifest.json"
echo.
echo Envie o game.log da pasta logs junto com o nome do onibus.
pause
