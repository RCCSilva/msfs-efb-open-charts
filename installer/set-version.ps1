<#
  Sets the user-facing version everywhere it appears, and regenerates the Windows resource
  (icon + version info) of the executable.

  Usage:  powershell -File installer\set-version.ps1 -Version 0.1.1 [-Label 'Open Beta']

  The version lives in the file VERSION at the repository root; this script writes it and syncs:
    core/cmd/opencharts/winres/winres.json  (+ regenerates rsrc_windows_amd64.syso)
    core/web/manager/manager.js             (VERSION shown in the footer and About)
    core/internal/setup/setup.go            (EFBVersion, the EFB package version)
    core/internal/setup/efbpkg/OpenCharts.js
    installer/OpenCharts.iss, installer/build.ps1 (defaults)
    README.md, AGENTS.md, issue template placeholder
#>
param(
  [Parameter(Mandatory = $true)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
  [string]$Label = 'Open Beta'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$utf8 = New-Object System.Text.UTF8Encoding($false)

function Update-File([string]$rel, [scriptblock]$edit) {
  $path = Join-Path $root $rel
  $text = [IO.File]::ReadAllText($path, $utf8)
  $new = & $edit $text
  if ($new -eq $text) { Write-Host "  unchanged  $rel" } else {
    [IO.File]::WriteAllText($path, $new, $utf8); Write-Host "  updated    $rel"
  }
}

$full = "$Version Open Beta"
if ($Label -ne 'Open Beta') { $full = if ($Label) { "$Version $Label" } else { $Version } }
$four = "$Version.0"

Write-Host "Version: $full"
[IO.File]::WriteAllText((Join-Path $root 'VERSION'), "$Version`n", $utf8)

Update-File 'core\cmd\opencharts\winres\winres.json' {
  param($t)
  $t = $t -replace '("identity":\s*\{[^}]*"version":\s*")[^"]*(")', "`${1}$four`${2}"
  $t = $t -replace '("file_version":\s*")[^"]*(")', "`${1}$four`${2}"
  $t = $t -replace '("product_version":\s*")[^"]*(")', "`${1}$four`${2}"
  $t = $t -replace '("FileVersion":\s*")[^"]*(")', "`${1}$Version`${2}"
  $t = $t -replace '("ProductVersion":\s*")[^"]*(")', "`${1}$full`${2}"
  $t
}
Update-File 'core\web\manager\manager.js' { param($t) $t -replace "((?:const|let) VERSION = ')[^']*(';)", "`${1}$full`${2}" }
Update-File 'core\internal\setup\setup.go' { param($t) $t -replace '(const EFBVersion = ")[^"]*(")', "`${1}$Version`${2}" }
Update-File 'core\internal\setup\efbpkg\OpenCharts.js' { param($t) $t -replace "(var VERSION = ')[^']*(';)", "`${1}$Version`${2}" }
Update-File 'installer\OpenCharts.iss' { param($t) $t -replace '(#define AppVersion ")[^"]*(")', "`${1}$Version`${2}" }
Update-File 'installer\build.ps1' {
  param($t)
  $t = $t -replace "(\[string\]\`$Version = ')[^']*(')", "`${1}$Version`${2}"
  $t -replace '(\[-Version )[^\]]*(\])', "`${1}$Version`${2}"
}
Update-File 'README.md' {
  param($t)
  $t = $t -replace '(# Open Charts . )\d+\.\d+\.\d+ Open Beta', "`${1}$full"
  $t -replace '(build\.ps1 -Version )\d+\.\d+\.\d+', "`${1}$Version"
}
Update-File 'AGENTS.md' {
  param($t)
  $t = $t -replace '\(\d+\.\d+\.\d+ Open Beta\)', "($full)"
  $t = $t -replace '(build\.ps1 -Version )\d+\.\d+\.\d+', "`${1}$Version"
  $t -replace '(user-facing version is `)[^`]*(`)', "`${1}$full`${2}"
}
Update-File '.github\ISSUE_TEMPLATE\bug_report.yml' { param($t) $t -replace '(placeholder: ")\d+\.\d+\.\d+[^"]*(")', "`${1}$full`${2}" }

Write-Host '== regenerate the executable resource (icon + version)'
Push-Location (Join-Path $root 'core\cmd\opencharts')
try { go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64; if ($LASTEXITCODE -ne 0) { throw 'go-winres failed' } }
finally { Pop-Location }
