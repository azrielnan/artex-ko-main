@echo off
setlocal
chcp 65001 >nul 2>&1
cd /d "%~dp0"
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-local.ps1" %*
set "ARTEX_RC=%ERRORLEVEL%"
if not "%ARTEX_RC%"=="0" (
  echo ARTEX 启动失败，请查看 data\launcher.log、data\postgres.log 和 data\artex.stderr.log
  pause
)
exit /b %ARTEX_RC%
