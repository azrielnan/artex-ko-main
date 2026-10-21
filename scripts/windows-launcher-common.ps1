# Windows PowerShell 5.1 helpers. Never log configuration or credentials.
function Quote-ArtexPS([string]$Value) { return "'" + $Value.Replace("'", "''") + "'" }
function Start-ArtexHidden([string]$Command, [switch]$Wait) {
  $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($Command))
  $process = Start-Process -FilePath "$env:WINDIR\System32\WindowsPowerShell\v1.0\powershell.exe" -ArgumentList @('-NoLogo','-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded) -WindowStyle Hidden -PassThru
  # Start-Process -Wait waits for the whole Windows process tree, including PostgreSQL.
  # Wait only for pg_ctl/the wrapper, while keeping its handle for a reliable exit code.
  $null = $process.Handle
  if ($Wait) { $process.WaitForExit() }
  return $process
}
function Read-ArtexPort([string]$Path, [int]$Fallback) {
  $portValue = 0
  if ((Test-Path -LiteralPath $Path) -and [int]::TryParse(([string](Get-Content -LiteralPath $Path -Raw)).Trim(), [ref]$portValue) -and $portValue -ge 1024 -and $portValue -le 65535) { return $portValue }
  return $Fallback
}
function Get-ArtexFreePort([int]$Preferred) {
  $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, $Preferred)
  $listener.Server.ExclusiveAddressUse = $true
  try { $listener.Start(); return $Preferred }
  catch [Net.Sockets.SocketException] {
    $fallback = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    try { $fallback.Start(); return $fallback.LocalEndpoint.Port } finally { $fallback.Stop() }
  } finally { $listener.Stop() }
}
function Get-ArtexReady([int]$Port) {
  $response = $null
  try {
    $request = [Net.HttpWebRequest]::Create("http://127.0.0.1:$Port/api/ready")
    $request.Proxy = $null
    $request.Timeout = 2000
    $request.ReadWriteTimeout = 2000
    $response = $request.GetResponse()
    $reader = [IO.StreamReader]::new($response.GetResponseStream())
    try { $result = $reader.ReadToEnd() | ConvertFrom-Json } finally { $reader.Dispose() }
    if ($result.service -eq 'artex') { return $result }
  } catch { return $null } finally { if ($response) { $response.Dispose() } }
}
function Test-ArtexProcess([int]$ProcessId, [string]$Executable) {
  if ($ProcessId -le 0) { return $false }
  try { return (Get-Process -Id $ProcessId -ErrorAction Stop).Path -eq $Executable } catch { return $false }
}
function Find-ArtexPostgres([string]$Root) {
  $candidates = @($env:ARTEX_PG_RUNTIME, (Join-Path $Root 'runtime\pgsql'), (Join-Path $Root '.tools\pgsql\pgsql'), (Join-Path (Split-Path $Root -Parent) 'pgsql-portable\pgsql'))
  foreach ($candidate in $candidates) {
    if ($candidate -and (Test-Path -LiteralPath (Join-Path $candidate 'bin\pg_ctl.exe'))) { return (Resolve-Path -LiteralPath $candidate).Path }
  }
  return $null
}
