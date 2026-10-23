$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$pidFile="$projectRoot/.tools/pangu-process.json"
if(!(Test-Path -LiteralPath $pidFile)){throw '没有本脚本登记的进程，请在原启动终端停止，不按名称批量结束进程'}
$record=Get-Content -LiteralPath $pidFile -Raw -Encoding UTF8 | ConvertFrom-Json
$process=Get-CimInstance Win32_Process -Filter "ProcessId = $([int]$record.pid)"
$allowed=@([IO.Path]::GetFullPath("$projectRoot/pangu.exe"),[IO.Path]::GetFullPath("$projectRoot/.tools/pangu.exe"))
if($process){
    if($record.binary -notin $allowed -or $process.ExecutablePath -ne $record.binary -or $process.CreationDate.ToUniversalTime().Ticks.ToString() -ne $record.started_ticks){throw '进程身份与启动记录不符，拒绝停止'}
    Stop-Process -Id $process.ProcessId -ErrorAction Stop
    Wait-Process -Id $process.ProcessId -Timeout 10 -ErrorAction SilentlyContinue
}
Remove-Item -LiteralPath $pidFile
Write-Output 'Pangu 已停止；Windows Job 会终止其原生子进程，在途任务由 Hatchet 保留，重启后按恢复规则处理。数据库与 Hatchet 未停止。'
