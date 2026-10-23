param([switch]$TestDatabase)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$fileName = if ($TestDatabase) { 'pangu-test-database.json' } else { 'pangu-database.json' }
$credentialPath = Join-Path $projectRoot ".tools/$fileName"
if (!(Test-Path -LiteralPath $credentialPath)) { throw '请先运行 database-init.ps1 创建独立数据库。' }
$credential = Get-Content -LiteralPath $credentialPath -Raw -Encoding UTF8 | ConvertFrom-Json
$postgresState = Get-Content -LiteralPath "$projectRoot/.tools/hatchet-postgres/data/postmaster.pid"
$databasePort = [int]$postgresState[3]
if ($databasePort -lt 1 -or $databasePort -gt 65535) { throw '本地 PostgreSQL 端口无效' }
$databaseUser = [uri]::EscapeDataString($credential.user)
$databasePassword = [uri]::EscapeDataString($credential.password)
$databaseName = [uri]::EscapeDataString($credential.database)
$databaseURL = "postgresql://${databaseUser}:${databasePassword}@127.0.0.1:${databasePort}/${databaseName}?sslmode=disable"
if ($TestDatabase) {
    $env:PANGU_TEST_DATABASE_URL = $databaseURL
} else {
    $env:PANGU_DATABASE_URL = $databaseURL
}
