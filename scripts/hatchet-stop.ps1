$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$pidPath = Join-Path $projectRoot '.tools/hatchet-host.pid'
if (!(Test-Path -LiteralPath $pidPath)) { throw '没有后台宿主 PID，请在启动终端使用 Ctrl+C。' }
$hostPIDValue = [int](Get-Content -LiteralPath $pidPath -Raw)
$hostProcess = Get-CimInstance Win32_Process -Filter "ProcessId = $hostPIDValue"
$scriptPath = Join-Path $PSScriptRoot 'hatchet.ps1'
if (!$hostProcess -or $hostProcess.CommandLine.IndexOf($scriptPath, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
    throw 'PID 不属于本项目 Hatchet 宿主，拒绝停止。'
}
Stop-Process -Id $hostPIDValue
Remove-Item -LiteralPath $pidPath
Write-Output '已关闭宿主，Hatchet 将通过标准输入关闭信号退出，数据库目录保留。'
