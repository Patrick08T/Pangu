param([switch]$Database)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$goBinary = if (Test-Path "$projectRoot/.tools/go/bin/go.exe") { "$projectRoot/.tools/go/bin/go.exe" } else { (Get-Command go -ErrorAction Stop).Source }

function Invoke-Check {
    param([string]$Program, [string[]]$CommandArguments)
    & $Program @CommandArguments
    if ($LASTEXITCODE -ne 0) { throw "检查失败：$Program $CommandArguments" }
}

$previousTestURL = $env:PANGU_TEST_DATABASE_URL
Push-Location -LiteralPath $projectRoot
try {
    if ($Database) { . "$PSScriptRoot/database-env.ps1" -TestDatabase }
    Invoke-Check 'powershell.exe' @('-NoProfile', '-File', "$PSScriptRoot/codex-source.test.ps1")
    Set-Location -LiteralPath "$projectRoot/web"
    Invoke-Check 'npm.cmd' @('test')
    Invoke-Check 'npm.cmd' @('run', 'build')
    Set-Location -LiteralPath $projectRoot
    Invoke-Check $goBinary @('test', '-p', '1', './...')
    Invoke-Check $goBinary @('vet', './...')
    Invoke-Check $goBinary @('build', './cmd/...')
} finally {
    $env:PANGU_TEST_DATABASE_URL = $previousTestURL
    Pop-Location
}
