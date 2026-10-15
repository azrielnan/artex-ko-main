@echo off
chcp 65001 >nul
cd /d "%~dp0"
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0启动.ps1"
if errorlevel 1 (
  echo.
  echo ARTEX 启动失败，请查看 data\artex.stderr.log 和 data\postgres.log
  pause
)
