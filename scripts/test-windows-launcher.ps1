param([string]$PackageRoot)
# Real PostgreSQL + embedded backend smoke test, isolated from the user's data/configuration.
$ErrorActionPreference = 'Stop'
if (-not $PackageRoot) { $PackageRoot = Split-Path $PSScriptRoot -Parent }
. (Join-Path $PSScriptRoot 'windows-launcher-common.ps1')
$source = [IO.Path]::GetFullPath($PackageRoot)
$fixture = Join-Path $source ("dist\启动 回归 ' " + [guid]::NewGuid().ToString('N'))
$testDataDir = Join-Path $env:TEMP ("artex-smoke-" + [guid]::NewGuid().ToString('N'))
$pgRoot = Find-ArtexPostgres $source
if (-not $pgRoot) { throw 'Set ARTEX_PG_RUNTIME to a PostgreSQL runtime before testing.' }
$previousRuntime = $env:ARTEX_PG_RUNTIME
$previousConfig = $env:ARTEX_CONFIG
$first = $null
$second = $null
$appBlocker = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, 0)
$dbBlocker = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, 0)
function Assert-Test([bool]$Condition, [string]$Message) {
  if (-not $Condition) { throw "FAIL: $Message" }
  Write-Output "PASS: $Message"
}
function Read-TestState { return Get-Content -LiteralPath (Join-Path $testDataDir 'launcher-state.json') -Raw | ConvertFrom-Json }
try {
  New-Item -ItemType Directory -Path (Join-Path $fixture 'scripts') -Force | Out-Null
  Copy-Item -LiteralPath (Join-Path $source 'artex.exe') -Destination $fixture
  # Exercise the real self-update bootstrap: checksum-verified swap -> exit 75 -> relaunch.
  Copy-Item -LiteralPath (Join-Path $source 'artex.exe') -Destination (Join-Path $fixture 'artex.new.exe')
  (Get-FileHash -LiteralPath (Join-Path $fixture 'artex.new.exe') -Algorithm SHA256).Hash.ToLowerInvariant() | Set-Content -LiteralPath (Join-Path $fixture 'artex.new.exe.sha256') -Encoding ASCII
  Copy-Item -LiteralPath (Join-Path $source 'start-local.ps1') -Destination $fixture
  foreach ($name in @('windows-launcher-common.ps1','windows-supervisor.ps1')) {
    Copy-Item -LiteralPath (Join-Path $source "scripts\$name") -Destination (Join-Path $fixture 'scripts')
  }
  $env:ARTEX_PG_RUNTIME = $pgRoot
  New-Item -ItemType Directory -Path $testDataDir -Force | Out-Null
  $env:ARTEX_CONFIG = Join-Path $testDataDir 'config.json'
  $appBlocker.Start(); $dbBlocker.Start()
  $blockedApp = $appBlocker.LocalEndpoint.Port
  $blockedDb = $dbBlocker.LocalEndpoint.Port
  # GitHub's hosted Windows image uses an ANSI code page that cannot represent
  # Chinese argv passed to initdb. Keep the package/install path Unicode (which
  # exercises the launcher and executable paths), but put the isolated PG data
  # path under TEMP so the native database utility receives representable argv.
  $command = '& ' + (Quote-ArtexPS (Join-Path $fixture 'start-local.ps1')) + ' -NoBrowser -DataDir ' + (Quote-ArtexPS $testDataDir) + " -Port $blockedApp -DatabasePort $blockedDb; exit " + '$LASTEXITCODE'
  $first = Start-ArtexHidden $command
  $second = Start-ArtexHidden $command
  Assert-Test ($first.WaitForExit(150000)) 'first launcher finishes'
  Assert-Test ($second.WaitForExit(150000)) 'concurrent launcher finishes'
  Assert-Test ($first.ExitCode -eq 0 -and $second.ExitCode -eq 0) 'fresh startup and concurrent double-click succeed when the package path contains Chinese/space/apostrophe characters'
  $state = Read-TestState
  $ready = Get-ArtexReady $state.port
  Assert-Test ($ready.ok -and $ready.database -eq 'ready') 'real backend and database ready after launchers exit'
  Assert-Test ((Get-Content -LiteralPath (Join-Path $testDataDir 'launcher.log') -Raw) -match 'exit=75') 'real staged self-update exit 75 is automatically relaunched'
  Assert-Test ($state.port -ne $blockedApp) 'HTTP port conflict falls back'
  $actualDb = Read-ArtexPort (Join-Path $testDataDir 'pg-port.txt') 0
  Assert-Test ($actualDb -ne $blockedDb -and $actualDb -ne 0) 'PostgreSQL port conflict falls back'
  $third = Start-ArtexHidden $command -Wait
  Assert-Test ($third.ExitCode -eq 0 -and (Read-TestState).app_pid -eq $state.app_pid) 'repeated startup reuses the same backend'
  # A stale cached port must not redirect the launcher to an unrelated database.
  $blockedDb | Set-Content -LiteralPath (Join-Path $testDataDir 'pg-port.txt') -Encoding ASCII
  Stop-Process -Id $state.app_pid -Force
  $deadline = [DateTime]::UtcNow.AddSeconds(40)
  do {
    Start-Sleep -Milliseconds 500
    $newState = Read-TestState
    $ready = Get-ArtexReady $state.port
  } while ((-not $ready -or $newState.app_pid -eq $state.app_pid) -and [DateTime]::UtcNow -lt $deadline)
  Assert-Test ($ready.ok -and $newState.app_pid -ne $state.app_pid) 'supervisor restarts an unexpectedly terminated backend'
  # Stop only our supervisor; leave our database alive to test PID-file port discovery.
  Stop-Process -Id $newState.supervisor_pid -Force
  Stop-Process -Id $newState.app_pid -Force
  $fourth = Start-ArtexHidden $command -Wait
  Assert-Test ($fourth.ExitCode -eq 0) 'existing PostgreSQL is reused despite stale port file'
  Assert-Test ((Read-ArtexPort (Join-Path $testDataDir 'pg-port.txt') 0) -eq $actualDb) 'running PostgreSQL actual port is restored'
  Write-Output "Artifacts: $fixture"
} finally {
  foreach ($launcher in @($first,$second)) {
    if ($launcher -and -not $launcher.HasExited) { Stop-Process -Id $launcher.Id -Force -ErrorAction SilentlyContinue }
  }
  if (Test-Path -LiteralPath (Join-Path $testDataDir 'launcher-state.json')) {
    $state = Read-TestState
    $supervisorProcess = Get-Process -Id $state.supervisor_pid -ErrorAction SilentlyContinue
    if ($supervisorProcess -and $supervisorProcess.ProcessName -eq 'powershell') { Stop-Process -Id $supervisorProcess.Id -Force }
    if (Test-ArtexProcess $state.app_pid (Join-Path $fixture 'artex.exe')) { Stop-Process -Id $state.app_pid -Force }
  }
  if (Test-Path -LiteralPath (Join-Path $testDataDir 'pgdata\PG_VERSION')) {
    $pgStop = '& ' + (Quote-ArtexPS (Join-Path $pgRoot 'bin\pg_ctl.exe')) + ' -D ' + (Quote-ArtexPS (Join-Path $testDataDir 'pgdata')) + ' -m fast -w stop; exit $LASTEXITCODE'
    Start-ArtexHidden $pgStop -Wait | Out-Null
  }
  $diagnosticDir = Join-Path $fixture 'data'
  New-Item -ItemType Directory -Path $diagnosticDir -Force | Out-Null
  foreach ($name in @('launcher.log','initdb.log','postgres.log','artex.stderr.log')) {
    $logPath = Join-Path $testDataDir $name
    if (Test-Path -LiteralPath $logPath) { Copy-Item -LiteralPath $logPath -Destination $diagnosticDir -Force }
  }
  $appBlocker.Stop(); $dbBlocker.Stop()
  $env:ARTEX_PG_RUNTIME = $previousRuntime
  $env:ARTEX_CONFIG = $previousConfig
}
