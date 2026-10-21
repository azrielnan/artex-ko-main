param([Parameter(Mandatory = $true)][string]$OptionsJson)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-launcher-common.ps1')
$options = $OptionsJson | ConvertFrom-Json
$root = $options.Root
$dataDir = $options.DataDir
$lock = $null
$log = Join-Path $dataDir 'launcher.log'
function Write-LauncherLog([string]$Message) {
  Add-Content -LiteralPath $log -Value "$(Get-Date -Format o) $Message" -Encoding UTF8
}
try {
  $lock = [IO.File]::Open((Join-Path $dataDir 'supervisor.lock'), 'OpenOrCreate', 'ReadWrite', 'None')
  $failures = 0
  while ($true) {
    foreach ($name in @('artex.stdout.log','artex.stderr.log')) {
      $path = Join-Path $dataDir $name
      if (Test-Path -LiteralPath $path) { Move-Item -LiteralPath $path -Destination "$path.previous" -Force }
    }
    $arguments = @('-addr', "127.0.0.1:$($options.Port)", '-data', $dataDir, '-proxy=') + @($options.ArtexArgs)
    $quoted = foreach ($argument in $arguments) {
      if ($null -ne $argument) { '"' + ([regex]::Replace([regex]::Replace([string]$argument, '(\\*)"', '$1$1\"'), '(\\+)$', '$1$1')) + '"' }
    }
    $child = Start-Process -FilePath (Join-Path $root 'artex.exe') -ArgumentList $quoted -WorkingDirectory $root -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $dataDir 'artex.stdout.log') -RedirectStandardError (Join-Path $dataDir 'artex.stderr.log')
    $null = $child.Handle
    @{ app_pid = $child.Id; supervisor_pid = $PID; port = $options.Port } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $dataDir 'launcher-state.json') -Encoding UTF8
    Write-LauncherLog "应用已启动，PID=$($child.Id)，端口=$($options.Port)。"
    $started = [DateTime]::UtcNow
    $child.WaitForExit()
    $code = $child.ExitCode
    Write-LauncherLog "应用退出，exit=$code。"
    if ($code -eq 0) { break }
    if ($code -eq 75) { $failures = 0; continue }
    if (([DateTime]::UtcNow - $started).TotalSeconds -gt 60) { $failures = 0 }
    $failures++
    if ($failures -ge 5) { throw '应用连续启动失败 5 次，停止重试；请检查 artex.stderr.log。' }
    Start-Sleep -Seconds ([Math]::Min(30, [Math]::Pow(2, $failures)))
  }
} catch { Write-LauncherLog $_.Exception.Message; exit 1 }
finally { if ($lock) { $lock.Dispose() } }
