param(
  [switch]$NoBrowser,
  [string]$DataDir,
  [ValidateRange(1024,65535)][int]$Port = 8787,
  [ValidateRange(1024,65535)][int]$DatabasePort = 5433,
  [Parameter(ValueFromRemainingArguments = $true)][string[]]$ArtexArgs
)
$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
. (Join-Path $root 'scripts\windows-launcher-common.ps1')
if (-not $DataDir) { $DataDir = Join-Path $root 'data' }
$DataDir = [IO.Path]::GetFullPath($DataDir)
New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
$lock = $null
try {
  $deadline = [DateTime]::UtcNow.AddSeconds(150)
  while (-not $lock) {
    try { $lock = [IO.File]::Open((Join-Path $DataDir 'startup.lock'), 'OpenOrCreate', 'ReadWrite', 'None') }
    catch [IO.IOException] {
      if ([DateTime]::UtcNow -gt $deadline) { throw '另一个启动程序长时间未完成，请查看 launcher.log。' }
      Start-Sleep -Milliseconds 300
    }
  }
  $exe = Join-Path $root 'artex.exe'
  if (-not (Test-Path -LiteralPath $exe)) { throw '找不到 artex.exe，请完整解压分发包，或先构建应用。' }
  $appPortFile = Join-Path $DataDir 'app-port.txt'
  $stateFile = Join-Path $DataDir 'launcher-state.json'
  $appPort = Read-ArtexPort $appPortFile $Port
  $ready = $null
  $state = $null
  if (Test-Path -LiteralPath $stateFile) {
    try { $state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json } catch { }
  }
  if ($state -and $state.port -eq $appPort -and (Test-ArtexProcess $state.app_pid $exe)) { $ready = Get-ArtexReady $appPort }
  if (-not ($ready -and $ready.ok -and $ready.database -eq 'ready')) {
    $pgRoot = Find-ArtexPostgres $root
    if (-not $pgRoot) { throw '找不到 PostgreSQL 运行时。请完整解压 Windows 分发包，或设置 ARTEX_PG_RUNTIME。' }
    $pgBin = Join-Path $pgRoot 'bin'
    $env:Path = "$pgBin;$env:Path"
    if (-not (Test-Path "$env:WINDIR\System32\vcruntime140.dll") -and -not (Test-Path "$pgBin\vcruntime140.dll")) { throw '缺少 VC++ x64 运行库，请安装 Microsoft VC++ 2015-2022 x64，或重新下载完整包。' }
    $pgData = Join-Path $DataDir 'pgdata'
    $pgLog = Join-Path $DataDir 'postgres.log'
    $pgPortFile = Join-Path $DataDir 'pg-port.txt'
    $pgPort = Read-ArtexPort $pgPortFile $DatabasePort
    if (-not (Test-Path (Join-Path $pgData 'PG_VERSION'))) {
      if ((Test-Path $pgData) -and (Get-ChildItem -LiteralPath $pgData -Force | Select-Object -First 1)) { throw 'pgdata 非空但缺少 PG_VERSION。已保留数据，请备份后检查，不能自动重新初始化。' }
      $initdbLog = Join-Path $DataDir 'initdb.log'
      # Keep native diagnostics: initdb can fail before postgres.log exists.
      $initdbOutput = @(& "$pgBin\initdb.exe" -D $pgData -U artex --auth=trust -E UTF8 --no-locale 2>&1)
      $initdbOutput | Add-Content -LiteralPath $initdbLog -Encoding UTF8
      if ($LASTEXITCODE -ne 0) { throw "PostgreSQL 初始化失败（$LASTEXITCODE），详见 initdb.log。" }
    }
    & "$pgBin\pg_ctl.exe" -D $pgData status *> $null
    if ($LASTEXITCODE -eq 0) {
      $pidLines = @(Get-Content -LiteralPath (Join-Path $pgData 'postmaster.pid'))
      $pgPort = [int]$pidLines[3]
    } else {
      $pgPort = Get-ArtexFreePort $pgPort
      $command = '& ' + (Quote-ArtexPS "$pgBin\pg_ctl.exe") + ' -D ' + (Quote-ArtexPS $pgData) + ' -o ' + (Quote-ArtexPS "-p $pgPort -h 127.0.0.1") + ' -l ' + (Quote-ArtexPS $pgLog) + ' -w -t 60 start; exit $LASTEXITCODE'
      $pgStart = Start-ArtexHidden $command -Wait
      if ($pgStart.ExitCode -ne 0) { throw "PostgreSQL 启动失败（$($pgStart.ExitCode)），请查看 $pgLog。" }
    }
    $pgPort | Set-Content -LiteralPath $pgPortFile -Encoding ASCII
    $env:PGCONNECT_TIMEOUT = '3'
    & "$pgBin\psql.exe" -h 127.0.0.1 -p $pgPort -U artex -d postgres -tAc 'SELECT 1' *> $null
    if ($LASTEXITCODE -ne 0) { throw '数据库进程存在但连接失败，请查看 postgres.log；不会删除 PID 文件或重建数据库。' }
    $dbExists = & "$pgBin\psql.exe" -h 127.0.0.1 -p $pgPort -U artex -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='artex'"
    if ($LASTEXITCODE -ne 0) { throw '数据库检查失败。' }
    if ("$dbExists".Trim() -ne '1') {
      & "$pgBin\createdb.exe" -h 127.0.0.1 -p $pgPort -U artex artex
      if ($LASTEXITCODE -ne 0) { throw "创建数据库失败（$LASTEXITCODE）。" }
    }
    $env:ARTEX_PG_DSN = "postgres://artex@127.0.0.1:$pgPort/artex?sslmode=disable"
    $env:ARTEX_SKILL_DIR = Join-Path $root 'skills'
    $supervisorActive = $false
    try {
      $probe = [IO.File]::Open((Join-Path $DataDir 'supervisor.lock'), 'OpenOrCreate', 'ReadWrite', 'None')
      $probe.Dispose()
    } catch [IO.IOException] { $supervisorActive = $true }
    if (-not $supervisorActive) {
      $appPort = Get-ArtexFreePort $appPort
      $appPort | Set-Content -LiteralPath $appPortFile -Encoding ASCII
      $options = @{ Root = $root; DataDir = $DataDir; Port = $appPort; ArtexArgs = @($ArtexArgs) } | ConvertTo-Json -Compress
      $command = '& ' + (Quote-ArtexPS (Join-Path $root 'scripts\windows-supervisor.ps1')) + ' -OptionsJson ' + (Quote-ArtexPS $options)
      $supervisor = Start-ArtexHidden $command
    }
    $deadline = [DateTime]::UtcNow.AddSeconds(120)
    do {
      $ready = Get-ArtexReady $appPort
      try { $state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json } catch { $state = $null }
      if ($ready -and $ready.ok -and $ready.database -eq 'ready' -and $state -and $state.port -eq $appPort -and (Test-ArtexProcess $state.app_pid $exe)) { break }
      if ($supervisor -and $supervisor.HasExited) { throw '后台守护程序已退出，请查看 launcher.log 和 artex.stderr.log。' }
      Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    if (-not ($ready -and $ready.ok -and $ready.database -eq 'ready' -and $state -and $state.port -eq $appPort -and (Test-ArtexProcess $state.app_pid $exe))) { throw '120 秒内未就绪，请查看 launcher.log、artex.stderr.log 和 postgres.log。' }
  }
  $appUrl = "http://127.0.0.1:$appPort"
  $appUrl | Set-Content -LiteralPath (Join-Path $DataDir 'url.txt') -Encoding ASCII
  Write-Output "ARTEX $($ready.version) 已就绪：$appUrl/function/tasks/"
  if (-not $NoBrowser) {
    try { Start-Process "$appUrl/function/tasks/" | Out-Null }
    catch { Write-Warning "浏览器未能自动打开，请手动访问：$appUrl/function/tasks/" }
  }
} catch {
  $message = "$(Get-Date -Format o) 启动失败：$($_.Exception.Message)"
  Add-Content -LiteralPath (Join-Path $DataDir 'launcher.log') -Value $message -Encoding UTF8
  Add-Content -LiteralPath (Join-Path $DataDir 'launcher.log') -Value $_.ScriptStackTrace -Encoding UTF8
  Write-Host $message -ForegroundColor Red
  exit 1
} finally { if ($lock) { $lock.Dispose() } }
