param([ValidateRange(1,65535)][int]$Port=8081, [ValidateSet('all','api','worker')][string]$Mode='all')
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$binary=if(Test-Path -LiteralPath "$projectRoot/pangu.exe"){"$projectRoot/pangu.exe"}else{"$projectRoot/.tools/pangu.exe"}
if(!(Test-Path -LiteralPath $binary)){throw '缺少 pangu.exe，请先构建或使用安装包'}
$binary=[IO.Path]::GetFullPath($binary)
[IO.Directory]::CreateDirectory("$projectRoot/.tools") | Out-Null
[IO.Directory]::CreateDirectory("$projectRoot/.artifacts") | Out-Null
$pidFile="$projectRoot/.tools/pangu-process.json"
if(Test-Path -LiteralPath $pidFile){
    $previous=Get-Content -LiteralPath $pidFile -Raw -Encoding UTF8 | ConvertFrom-Json
    $running=Get-CimInstance Win32_Process -Filter "ProcessId = $([int]$previous.pid)"
    if($running -and $running.ExecutablePath -eq $previous.binary -and $running.CreationDate.ToUniversalTime().Ticks.ToString() -eq $previous.started_ticks){throw 'Pangu 已启动，请先运行 stop.ps1'}
    Remove-Item -LiteralPath $pidFile
}
if($Mode -ne 'worker' -and (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)){throw "端口 $Port 已占用"}
$changedNames=@('HATCHET_CLIENT_TOKEN','HATCHET_CLIENT_TENANT_ID','HATCHET_CLIENT_HOST_PORT','HATCHET_CLIENT_SERVER_URL','HATCHET_CLIENT_TLS_STRATEGY','HATCHET_CLIENT_LOG_LEVEL','HATCHET_CLIENT_NAMESPACE','PANGU_DATABASE_URL','PANGU_WORKSPACE_ROOT','PANGU_AUDIT_SPOOL_ROOT','PANGU_PUBLIC_URL','PANGU_DATABASE_ADMIN_URL','PANGU_CODEX_BINARY','PANGU_CODEX_HOME')
$previousEnvironment=@{}
foreach($name in $changedNames){$previousEnvironment[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
try {
. "$PSScriptRoot/hatchet-env.ps1"
. "$PSScriptRoot/database-env.ps1"
if(!$env:HATCHET_CLIENT_NAMESPACE){$env:HATCHET_CLIENT_NAMESPACE='pangu'}
if(!$env:PANGU_WORKSPACE_ROOT){$env:PANGU_WORKSPACE_ROOT="$projectRoot/.artifacts/workspaces"}
if(!$env:PANGU_AUDIT_SPOOL_ROOT){$env:PANGU_AUDIT_SPOOL_ROOT="$projectRoot/.artifacts/audit"}
if(!$env:PANGU_PUBLIC_URL){$env:PANGU_PUBLIC_URL="http://127.0.0.1:$Port"}
if(!$env:PANGU_CODEX_BINARY -and (Test-Path -LiteralPath "$projectRoot/.tools/codex/codex.exe")){$env:PANGU_CODEX_BINARY="$projectRoot/.tools/codex/codex.exe"}
if(!$env:PANGU_CODEX_HOME){$env:PANGU_CODEX_HOME="$projectRoot/.tools/codex-home"}
foreach($directory in @($env:PANGU_WORKSPACE_ROOT,$env:PANGU_AUDIT_SPOOL_ROOT,$env:PANGU_CODEX_HOME)){
    if(![IO.Path]::IsPathRooted($directory)){throw 'Worker 存储与审计补录目录必须为绝对路径'}
    [IO.Directory]::CreateDirectory($directory) | Out-Null
}
    $env:PANGU_DATABASE_ADMIN_URL=$null
    $process=Start-Process -FilePath $binary -ArgumentList @('-mode',$Mode,'-port',$Port) -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput "$projectRoot/.artifacts/pangu.stdout.log" -RedirectStandardError "$projectRoot/.artifacts/pangu.stderr.log"
} finally {foreach($name in $changedNames){[Environment]::SetEnvironmentVariable($name,$previousEnvironment[$name],'Process')}}
$identity=Get-CimInstance Win32_Process -Filter "ProcessId = $($process.Id)"
if(!$identity){throw 'Pangu 启动失败，请检查 .artifacts/pangu.stderr.log'}
@{pid=$process.Id; binary=$binary; started_ticks=$identity.CreationDate.ToUniversalTime().Ticks.ToString(); mode=$Mode; port=$Port} | ConvertTo-Json | Set-Content -LiteralPath $pidFile -Encoding UTF8
Start-Sleep -Milliseconds 500
if($process.HasExited){throw 'Pangu 启动失败，请检查数据库迁移与服务配置'}
Write-Output "Pangu 进程已启动，PID=$($process.Id)，模式=$Mode，端口=$Port；就绪状态请检查 /api/readyz 与 .artifacts/pangu.stderr.log。"
