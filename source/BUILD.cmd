@echo off
setlocal EnableExtensions DisableDelayedExpansion
pushd "%~dp0.."
where py >nul 2>&1
if errorlevel 1 goto python
py -3 scripts\build.py %*
set "BUILD_RESULT=%ERRORLEVEL%"
goto done
:python
python scripts\build.py %*
set "BUILD_RESULT=%ERRORLEVEL%"
:done
popd
exit /b %BUILD_RESULT%
