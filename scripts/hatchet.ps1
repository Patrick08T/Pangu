param([switch]$Background, [ValidateRange(1,65535)][int]$GRPCPort=17070, [ValidateRange(1,65535)][int]$APIPort=28243)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
New-Item -ItemType Directory -Path '.tools', '.artifacts' -Force | Out-Null

if ($Background -and (Test-Path '.tools/hatchet-host.pid')) {
    $existingPID = [int](Get-Content '.tools/hatchet-host.pid' -Raw)
    $existingHost = Get-CimInstance Win32_Process -Filter "ProcessId = $existingPID"
    if ($existingHost -and $existingHost.CommandLine.IndexOf($PSCommandPath, [StringComparison]::OrdinalIgnoreCase) -ge 0) {
        throw '本项目 Hatchet 宿主已启动，请先使用停止脚本关闭。'
    }
}

if ($GRPCPort -eq $APIPort) { throw 'Hatchet 的 gRPC 和 API 端口必须不同' }
foreach ($port in @($GRPCPort, $APIPort)) {
    if (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) {
        throw "端口 $port 已占用，请先停止现有实例。"
    }
}

if ($Background) {
    $arguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ('"' + $PSCommandPath + '"'), '-GRPCPort', $GRPCPort, '-APIPort', $APIPort)
    $hostProcess = Start-Process powershell.exe -ArgumentList $arguments -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput "$projectRoot/.artifacts/hatchet.stdout.log" `
        -RedirectStandardError "$projectRoot/.artifacts/hatchet.stderr.log"
    $hostProcess.Id | Set-Content '.tools/hatchet-host.pid'
    Write-Output "Hatchet 启动中，宿主 PID=$($hostProcess.Id)，日志位于 .artifacts/hatchet.stderr.log"
    exit 0
}

if (!(Test-Path -LiteralPath '.tools/sidecar.exe')) {
    $goBinary = if (Test-Path '.tools/go/bin/go.exe') { "$projectRoot/.tools/go/bin/go.exe" } else { (Get-Command go).Source }
    $previousGoBin = $env:GOBIN
    try {
        $env:GOBIN = "$projectRoot/.tools"
        & $goBinary install github.com/hatchet-dev/hatchet-embedded/cmd/sidecar@v0.110.5
        if ($LASTEXITCODE -ne 0) { throw 'Hatchet 编译失败' }
    } finally { $env:GOBIN = $previousGoBin }
}

$passwordPath = "$projectRoot/.tools/hatchet-admin-password"
if (!(Test-Path -LiteralPath $passwordPath)) {
    [System.IO.File]::WriteAllText($passwordPath, [guid]::NewGuid().ToString('N') + '!aA1')
}
$env:ADMIN_PASSWORD = [System.IO.File]::ReadAllText($passwordPath)
# 嵌入式引擎有多个连接池，限制每池大小，为独立产品库保留连接。
$env:DATABASE_MAX_CONNS = '10'
$env:DATABASE_DDL_POOL_MAX_CONNS = '2'
if (Test-Path '.tools/hatchet-connection.json') { Remove-Item -LiteralPath '.tools/hatchet-connection.json' }

$engine = [System.Diagnostics.Process]::new()
$engine.StartInfo.FileName = "$projectRoot/.tools/sidecar.exe"
$engine.StartInfo.Arguments = "-grpc-port $GRPCPort -api-port $APIPort -postgres-data-dir .tools/hatchet-postgres -handshake-file .tools/hatchet-connection.json -log-level warn"
$engine.StartInfo.WorkingDirectory = $projectRoot
$engine.StartInfo.UseShellExecute = $false
$engine.StartInfo.CreateNoWindow = $true
$engine.StartInfo.RedirectStandardInput = $true
$engineHome = "$projectRoot/.tools/hatchet-home"
[IO.Directory]::CreateDirectory($engineHome) | Out-Null
$engine.StartInfo.EnvironmentVariables['USERPROFILE'] = $engineHome
$engine.StartInfo.EnvironmentVariables['HOME'] = $engineHome
$engine.StartInfo.EnvironmentVariables['SERVER_ENABLE_DATA_RETENTION'] = 'true'
$engine.StartInfo.EnvironmentVariables['SERVER_LIMITS_DEFAULT_TENANT_RETENTION_PERIOD'] = '720h'
$engine.StartInfo.EnvironmentVariables['SERVER_LIMITS_CORE_PARTITION_RETENTION'] = '720h'
$engine.StartInfo.EnvironmentVariables['SERVER_LIMITS_OLAP_PARTITION_RETENTION'] = '720h'
try {
    if (!$engine.Start()) { throw 'Hatchet 启动失败' }
    $engine.WaitForExit()
    if ($engine.ExitCode -ne 0) { throw "Hatchet 退出码 $($engine.ExitCode)" }
} finally {
    if ($engine.Id -and !$engine.HasExited) {
        $engine.StandardInput.Close()
        if (!$engine.WaitForExit(30000)) { Write-Warning 'Hatchet 仍在清理，请检查日志。' }
    }
    $engine.Dispose()
}
