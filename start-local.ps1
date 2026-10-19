param(
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ArtexArgs
)

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
$dataDir = Join-Path $root "data"
$pgData = Join-Path $dataDir "pgdata"
$pgLog = Join-Path $dataDir "postgres.log"
$appOut = Join-Path $dataDir "artex.stdout.log"
$appErr = Join-Path $dataDir "artex.stderr.log"
$pgPortFile = Join-Path $dataDir "pg-port.txt"
$appPortFile = Join-Path $dataDir "app-port.txt"
New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

function Test-PortInUse([int]$Port) {
  $connection = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1
  return $null -ne $connection
}

function Get-FreePort([int]$Preferred) {
  if (-not (Test-PortInUse $Preferred)) { return $Preferred }
  $listener = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, 0)
  try {
    $listener.Start()
    return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
  } finally {
    $listener.Stop()
  }
}

function Read-Port([string]$Path, [int]$Fallback) {
  if (Test-Path $Path) {
    $value = (Get-Content $Path -Raw).Trim()
    $port = 0
    if ([int]::TryParse($value, [ref]$port) -and $port -ge 1024 -and $port -le 65535) {
      return $port
    }
  }
  return $Fallback
}

function Find-PostgresRoot {
  $candidates = @()
  if ($env:ARTEX_PG_RUNTIME) { $candidates += $env:ARTEX_PG_RUNTIME }
  $candidates += (Join-Path $root "runtime\pgsql")
  $candidates += (Join-Path $root ".tools\pgsql\pgsql")
  # 仅用于当前源码树开发；发布包不会依赖这个绝对路径。
  $candidates += "E:\777\pgsql-portable\pgsql"
  foreach ($candidate in $candidates) {
    if ($candidate -and (Test-Path (Join-Path $candidate "bin\pg_ctl.exe"))) {
      return (Resolve-Path $candidate).Path
    }
  }
  return $null
}

function Invoke-ReadyRequest {
  try {
    return Invoke-RestMethod -Uri "http://127.0.0.1:$appPort/api/ready" -TimeoutSec 3
  } catch {
    return $null
  }
}

if (-not (Test-Path (Join-Path $root "artex.exe"))) {
  throw "找不到 artex.exe，请先构建应用。"
}

$pgRoot = Find-PostgresRoot
if (-not $pgRoot) {
  throw "找不到便携式 PostgreSQL。请确认分发包包含 runtime\pgsql，或设置 ARTEX_PG_RUNTIME。"
}
$pgBin = Join-Path $pgRoot "bin"
$pgPort = Read-Port $pgPortFile 5433
if (-not (Test-Path (Join-Path $pgData "PG_VERSION"))) {
  $pgPort = Get-FreePort $pgPort
}
$pgPort | Set-Content $pgPortFile -Encoding ASCII
$dsn = "postgres://artex@127.0.0.1:$pgPort/artex?sslmode=disable"

# 优先使用包内 VC 运行库；只有开发机和旧外部安装缺少它时才尝试 winget。
$vcRuntime = Join-Path $env:WINDIR "System32\vcruntime140.dll"
$bundledVcRuntime = Join-Path $pgBin "vcruntime140.dll"
if (-not (Test-Path $vcRuntime) -and -not (Test-Path $bundledVcRuntime)) {
  $winget = Get-Command winget.exe -ErrorAction SilentlyContinue
  if ($winget) {
    & $winget.Source install --id Microsoft.VCRedist.2015+.x64 --exact --silent --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) { throw "VC++ 2015-2022 x64 运行库安装失败（$LASTEXITCODE）。" }
  } else {
    throw "缺少 VC++ 2015-2022 x64 运行库，且系统没有 winget；请把 vcruntime140.dll 放入 runtime\pgsql\bin 后重试。"
  }
}

$env:Path = "$pgBin;$env:Path"

if (-not (Test-Path (Join-Path $pgData "PG_VERSION"))) {
  & (Join-Path $pgBin "initdb.exe") -D $pgData -U artex --auth=trust -E UTF8 --no-locale
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL 初始化失败（$LASTEXITCODE）。" }
}

& (Join-Path $pgBin "pg_ctl.exe") -D $pgData status *> $null
if ($LASTEXITCODE -ne 0) {
  if (Test-PortInUse $pgPort) {
    $pgPort = Get-FreePort $pgPort
    $pgPort | Set-Content $pgPortFile -Encoding ASCII
    $dsn = "postgres://artex@127.0.0.1:$pgPort/artex?sslmode=disable"
  }
  & (Join-Path $pgBin "pg_ctl.exe") -D $pgData -o "-p $pgPort -h 127.0.0.1" -l $pgLog start
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL 启动失败，请查看 data\postgres.log。" }
}

$pgReady = $false
for ($attempt = 0; $attempt -lt 30; $attempt++) {
  & (Join-Path $pgBin "psql.exe") -h 127.0.0.1 -p $pgPort -U artex -d postgres -tAc "SELECT 1" *> $null
  if ($LASTEXITCODE -eq 0) { $pgReady = $true; break }
  Start-Sleep -Seconds 1
}
if (-not $pgReady) { throw "PostgreSQL 在 30 秒内没有就绪。" }

$dbExists = & (Join-Path $pgBin "psql.exe") -h 127.0.0.1 -p $pgPort -U artex -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='artex'"
$dbExistsText = if ($null -eq $dbExists) { "" } else { [string]$dbExists }
if ($dbExistsText.Trim() -ne "1") {
  & (Join-Path $pgBin "createdb.exe") -h 127.0.0.1 -p $pgPort -U artex artex
  if ($LASTEXITCODE -ne 0) { throw "无法创建 artex 数据库（$LASTEXITCODE）。" }
}

$env:ARTEX_PG_DSN = $dsn
$env:ARTEX_SKILL_DIR = Join-Path $root "skills"
$appPort = Read-Port $appPortFile 8787
if (Test-PortInUse $appPort) {
  $existing = Get-NetTCPConnection -State Listen -LocalPort $appPort -ErrorAction SilentlyContinue | Select-Object -First 1
  $existingReady = Invoke-ReadyRequest
  if (-not ($existingReady -and $existingReady.ok)) {
    $appPort = Get-FreePort $appPort
  }
}
$appPort | Set-Content $appPortFile -Encoding ASCII
$appUrl = "http://127.0.0.1:$appPort"
$ready = Invoke-ReadyRequest
if (-not $ready -or -not $ready.ok) {
  $listener = Get-NetTCPConnection -State Listen -LocalPort 8787 -ErrorAction SilentlyContinue | Select-Object -First 1
  if ($listener) {
    throw "127.0.0.1:8787 已被其他程序占用，且不是可用的 ARTEX 服务。"
  }
  $args = @("-addr", "127.0.0.1:$appPort", "-proxy=")
  if ($ArtexArgs) { $args += $ArtexArgs }
  Start-Process -FilePath (Join-Path $root "artex.exe") `
    -ArgumentList $args -WorkingDirectory $root `
    -RedirectStandardOutput $appOut -RedirectStandardError $appErr -WindowStyle Hidden | Out-Null
}

for ($attempt = 0; $attempt -lt 60; $attempt++) {
  $ready = Invoke-ReadyRequest
  if ($ready -and $ready.ok -and $ready.database -eq "ready") {
    Write-Output "ARTEX 已就绪：$appUrl"
    Write-Output "数据库已就绪：127.0.0.1:$pgPort/artex"
    try {
      Start-Process "$appUrl/function/tasks" | Out-Null
      Write-Output "已打开浏览器任务页。"
    } catch {
      Write-Warning "浏览器未能自动打开，请手动访问：$appUrl/function/tasks"
    }
    exit 0
  }
  Start-Sleep -Seconds ([Math]::Min(2, 1 + [int]($attempt / 15)))
}
throw "ARTEX 在 120 秒内没有就绪，请查看 data\artex.stderr.log 和 data\postgres.log。"
