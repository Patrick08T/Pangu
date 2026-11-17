param(
    [string]$P4Binary = 'p4',
    [string]$P4D = '',
    [switch]$Unicode
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (!$P4D) { $P4D = Join-Path $projectRoot '.tools/p4/p4d.exe' }
$P4D = (Resolve-Path -LiteralPath $P4D).Path
$P4Binary = (Get-Command $P4Binary -ErrorAction Stop).Source
$serverRoot = Join-Path $projectRoot ('.tools/p4/test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $serverRoot | Out-Null
$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = $listener.LocalEndpoint.Port
$listener.Stop()
$previousPort = $env:PANGU_TEST_P4_PORT
$previousBinary = $env:PANGU_P4_BINARY
$previousDatabase = $env:PANGU_TEST_DATABASE_URL
$server = $null
Push-Location -LiteralPath $serverRoot
try {
    if ($Unicode) {
        & $P4D -r $serverRoot -xi
        if ($LASTEXITCODE -ne 0) { throw '初始化 Unicode 测试服务失败' }
    }
    $server = Start-Process -FilePath $P4D -ArgumentList @('-r',('"'+$serverRoot+'"'),'-p',("127.0.0.1:$port"),'-L','server.log','-J','journal') -WorkingDirectory $serverRoot -WindowStyle Hidden -PassThru
    $env:PANGU_TEST_P4_PORT = "127.0.0.1:$port"
    $env:PANGU_P4_BINARY = $P4Binary
    $ready = $false
    for ($attempt=0; $attempt -lt 40; $attempt++) {
        & $P4Binary -p $env:PANGU_TEST_P4_PORT -u tester -c probe info 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { $ready=$true; break }
        Start-Sleep -Milliseconds 100
    }
    if (!$ready) { throw '本地 P4 测试服务未就绪' }
    Set-Location -LiteralPath $projectRoot
    . "$PSScriptRoot/database-env.ps1" -TestDatabase
    & "$projectRoot/.tools/go/bin/go.exe" test -p 1 ./internal/perforce ./internal/workspace -run 'TestPerforce|TestRejectUnsafe|TestRemap|TestMalformed' -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw 'Perforce 集成测试失败' }
} finally {
    if ($server -and !$server.HasExited) { Stop-Process -Id $server.Id; Wait-Process -Id $server.Id -ErrorAction SilentlyContinue }
    $env:PANGU_TEST_P4_PORT = $previousPort
    $env:PANGU_P4_BINARY = $previousBinary
    $env:PANGU_TEST_DATABASE_URL = $previousDatabase
    Pop-Location
}
Write-Output "独立 P4 验证完成，服务已停止；日志保留：$serverRoot"
