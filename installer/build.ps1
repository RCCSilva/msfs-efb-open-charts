<#
  Builds Open Charts and, when Inno Setup 6 is installed, the installer.

  Usage:  powershell -File installer\build.ps1 [-Version 0.8.0]

  Output: build\opencharts.exe, build\SimConnect.dll and dist\OpenCharts-Setup-<version>.exe
#>
param([string]$Version = '0.8.0', [string]$Label = 'Open Beta')

$ErrorActionPreference = 'Stop'
$root  = Split-Path -Parent $PSScriptRoot
$build = Join-Path $root 'build'
$dist  = Join-Path $root 'dist'
New-Item -ItemType Directory -Force $build, $dist | Out-Null

Write-Host '== go build (no cgo)'
Push-Location (Join-Path $root 'core')
try {
  $env:CGO_ENABLED = '0'
  go test ./... | Out-Host
  if ($LASTEXITCODE -ne 0) { throw 'tests failed' }
  # The public ChartFox client id (not a secret) comes from the ignored .env file at the repository root.
  $ldflags = '-s -w -H windowsgui -X github.com/RCCSilva/msfs-efb-open-charts/core/internal/buildinfo.Release=1'
  $envFile = Join-Path $root '.env'
  if (Test-Path -LiteralPath $envFile) {
    $line = Get-Content -LiteralPath $envFile | Where-Object { $_ -match '^\s*CHARTFOX_CLIENT_ID\s*=' } | Select-Object -First 1
    if ($line) { $id = ($line -split '=', 2)[1].Trim().Trim('"'); if ($id) { $ldflags += " -X github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox.ClientID=$id" } }
  }
  go build -trimpath -ldflags $ldflags -o (Join-Path $build 'opencharts.exe') ./cmd/opencharts
  if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
} finally { Pop-Location }

Write-Host '== SimConnect.dll'
$dllCandidates = @(
  (Join-Path $build 'SimConnect.dll'),
  "$env:MSFS2024_SDK\SimConnect SDK\lib\SimConnect.dll",
  'C:\MSFS 2024 SDK\SimConnect SDK\lib\SimConnect.dll'
)
$dll = $dllCandidates | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -First 1
if (-not $dll) { throw 'SimConnect.dll not found: install the MSFS 2024 SDK or copy the DLL into build\' }
if ($dll -ne (Join-Path $build 'SimConnect.dll')) { Copy-Item -LiteralPath $dll -Destination $build -Force }

Write-Host '== portable zip'
$stage = Join-Path $build 'OpenCharts'
Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $stage | Out-Null
Copy-Item (Join-Path $build 'opencharts.exe'), (Join-Path $build 'SimConnect.dll') -Destination $stage
Copy-Item (Join-Path $root 'LICENSE') -Destination $stage
Copy-Item (Join-Path $PSScriptRoot 'readme-portable.txt') -Destination (Join-Path $stage 'README.txt')
$zip = Join-Path $dist "OpenCharts-$Version-open-beta-win64-portable.zip"
Remove-Item -LiteralPath $zip -Force -ErrorAction SilentlyContinue
Compress-Archive -Path $stage -DestinationPath $zip

Write-Host '== installer'
$iscc = @(
  "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
  "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
  "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
if (-not $iscc) {
  Write-Warning 'Inno Setup 6 not found. Install it (winget install JRSoftware.InnoSetup) and run this script again.'
  exit 0
}
& $iscc "/DAppVersion=$Version" "/DAppLabel=$Label" (Join-Path $PSScriptRoot 'OpenCharts.iss')
if ($LASTEXITCODE -ne 0) { throw 'ISCC failed' }
Get-ChildItem $dist -Filter 'OpenCharts-*' | Select-Object Name, Length
