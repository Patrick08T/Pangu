param([switch]$TestDatabase)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (!$env:PANGU_DATABASE_ADMIN_URL) { throw '请显式设置 PANGU_DATABASE_ADMIN_URL，仅供本次建库使用。' }
$fileName = if ($TestDatabase) { 'pangu-test-database.json' } else { 'pangu-database.json' }
$credentialPath = Join-Path $projectRoot ".tools/$fileName"
if (!(Test-Path -LiteralPath $credentialPath)) {
    New-Item -ItemType Directory -Path "$projectRoot/.tools" -Force | Out-Null
    $randomBytes = New-Object byte[] 32
    $random = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $random.GetBytes($randomBytes) } finally { $random.Dispose() }
    $credential = @{
        user = $(if ($TestDatabase) { 'pangu_test_app' } else { 'pangu_app' })
        database = $(if ($TestDatabase) { 'pangu_test' } else { 'pangu' })
        password = [BitConverter]::ToString($randomBytes).Replace('-', '').ToLowerInvariant()
    }
    [IO.File]::WriteAllText($credentialPath, ($credential | ConvertTo-Json))
}
$previousURL = $env:PANGU_DATABASE_URL
$packaged = (Test-Path -LiteralPath "$projectRoot/database.exe") -and (Test-Path -LiteralPath "$projectRoot/pangu.exe")
if (!$packaged) { $goBinary = if (Test-Path "$projectRoot/.tools/go/bin/go.exe") { "$projectRoot/.tools/go/bin/go.exe" } else { (Get-Command go -ErrorAction Stop).Source } }
Push-Location -LiteralPath $projectRoot
try {
    . "$PSScriptRoot/database-env.ps1" -TestDatabase:$TestDatabase
    if ($TestDatabase) { $env:PANGU_DATABASE_URL = $env:PANGU_TEST_DATABASE_URL }
    if ($packaged) { & "$projectRoot/database.exe" } else { & $goBinary run ./cmd/database }
    if ($LASTEXITCODE -ne 0) { throw '数据库初始化失败；已保存的随机密码保留，未自动覆盖账号。' }
    if ($packaged) { & "$projectRoot/pangu.exe" -mode migrate } else { & $goBinary run ./cmd/pangu -mode migrate }
    if ($LASTEXITCODE -ne 0) { throw '数据库迁移失败' }
} finally {
    $env:PANGU_DATABASE_URL = $previousURL
    Pop-Location
}
