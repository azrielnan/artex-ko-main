$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
& (Join-Path $root "start-local.ps1")
if ($LASTEXITCODE -ne 0) {
  throw "ARTEX startup failed ($LASTEXITCODE)."
}
Start-Process "http://127.0.0.1:8787/"
