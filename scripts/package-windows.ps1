param(
  [Parameter(Mandatory = $true)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
  [Parameter(Mandatory = $true)][string]$PostgresRuntime,
  [Parameter(Mandatory = $true)][string]$ReaRuntime,
  [string]$Binary
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
if (-not $Binary) { $Binary = Join-Path $root 'artex.exe' }
$packageName = "artex-$Version-windows-amd64"
$destination = Join-Path $root "dist\$packageName"
$archive = "$destination.zip"
if ((Test-Path -LiteralPath $destination) -or (Test-Path -LiteralPath $archive)) { throw 'Package output already exists; choose a new version or preserve/move the previous output first.' }
foreach ($required in @($Binary, "$PostgresRuntime\bin\pg_ctl.exe", "$ReaRuntime\node.exe", "$ReaRuntime\node_modules\rea-agents\scripts\rea.mjs")) {
  if (-not (Test-Path -LiteralPath $required)) { throw "Missing runtime component: $required" }
}
New-Item -ItemType Directory -Path "$destination\scripts","$destination\runtime\pgsql" -Force | Out-Null
# Explicit whitelist: never copy the repository root, configuration, database or JWT key.
Copy-Item -LiteralPath $Binary -Destination "$destination\artex.exe"
foreach ($name in @('start.bat','start-local.ps1','启动.bat','启动.ps1','config.example.json','README.md','README.zh.md','CHANGELOG.zh.md','LICENSE')) {
  Copy-Item -LiteralPath (Join-Path $root $name) -Destination $destination
}
foreach ($name in @('windows-launcher-common.ps1','windows-supervisor.ps1')) {
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination "$destination\scripts"
}
Copy-Item -LiteralPath "$root\skills" -Destination $destination -Recurse
foreach ($name in @('bin','lib','share')) {
  Copy-Item -LiteralPath (Join-Path $PostgresRuntime $name) -Destination "$destination\runtime\pgsql" -Recurse
}
Get-ChildItem -LiteralPath $PostgresRuntime -File | Where-Object Name -match 'license|copyright|notice' | Copy-Item -Destination "$destination\runtime\pgsql"
foreach ($name in @('vcruntime140.dll','vcruntime140_1.dll')) {
  if (-not (Test-Path -LiteralPath "$destination\runtime\pgsql\bin\$name")) {
    $dll = Get-ChildItem -LiteralPath $PostgresRuntime -Filter $name -Recurse | Select-Object -First 1
    if (-not $dll) { throw "Missing redistributable: $name" }
    Copy-Item -LiteralPath $dll.FullName -Destination "$destination\runtime\pgsql\bin"
  }
}
Copy-Item -LiteralPath $ReaRuntime -Destination "$destination\runtime\rea" -Recurse
$pgVersion = & "$PostgresRuntime\bin\postgres.exe" --version
@"
ARTEX $Version
target=windows/amd64
postgres=$pgVersion
rea_mcp=bundled
startup=start.bat / 启动.bat
secrets=config.json,jwt.key,data/ intentionally excluded
"@ | Set-Content -LiteralPath "$destination\RELEASE-MANIFEST.txt" -Encoding UTF8
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -AssemblyName System.IO.Compression
$outputZip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Create)
try {
  foreach ($file in (Get-ChildItem -LiteralPath $destination -File -Recurse -Force)) {
    # .NET Framework CreateFromDirectory uses backslashes; ZIP names must use '/'.
    $relativeName = $file.FullName.Substring($destination.Length + 1).Replace('\','/')
    [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($outputZip, $file.FullName, "$packageName/$relativeName", [IO.Compression.CompressionLevel]::Optimal) | Out-Null
  }
} finally { $outputZip.Dispose() }
$zip = [IO.Compression.ZipFile]::OpenRead($archive)
try {
  foreach ($forbidden in @('config.json','jwt.key','data/')) {
    if ($zip.Entries.FullName -contains "$packageName/$forbidden" -or $zip.Entries.FullName -like "$packageName/data/*") { throw "Unexpected private artifact in package: $forbidden" }
  }
} finally { $zip.Dispose() }
$hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  $packageName.zip" | Set-Content -LiteralPath (Join-Path $root 'dist\SHA256SUMS-windows-amd64') -Encoding ASCII
Write-Output "Package: $archive"
Write-Output "SHA256: $hash"
