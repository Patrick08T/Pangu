param([Parameter(Mandatory=$true)][string]$Backup, [Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/backup-files.ps1"
$Backup = [IO.Path]::GetFullPath($Backup)
$Destination = [IO.Path]::GetFullPath($Destination)
if (Test-Path -LiteralPath $Destination) { throw '恢复目标已存在，禁止覆盖原数据' }
Get-BackupTree $Backup | Out-Null
if ($Destination.StartsWith(($Backup.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar), [StringComparison]::OrdinalIgnoreCase)) { throw '恢复目标不能位于备份内' }
$manifest = Get-Content -LiteralPath "$Backup/manifest.json" -Raw -Encoding UTF8 | ConvertFrom-Json
if ($manifest.format -ne 'pangu-cold-backup-v1' -or !$manifest.files -or !$manifest.directories) { throw '备份格式无效' }
$seen = @{}
foreach ($file in $manifest.files) {
    $source = Resolve-BackupPath $Backup $file.path
    if ($seen.ContainsKey($source) -or $file.sha256 -notmatch '^[A-Fa-f0-9]{64}$') { throw '备份清单重复或摘要无效' }
    $seen[$source] = $true
    $item = Get-Item -LiteralPath $source -Force
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw '备份文件类型无效' }
    if ($item.Length -ne $file.bytes -or (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ne $file.sha256) { throw '备份文件已损坏或被修改，停止恢复' }
}
foreach ($directory in $manifest.directories) { Resolve-BackupPath $Destination $directory | Out-Null }
New-PrivateBackupDirectory $Destination
foreach ($directory in $manifest.directories) { [IO.Directory]::CreateDirectory((Resolve-BackupPath $Destination $directory)) | Out-Null }
foreach ($file in $manifest.files) {
    $target = Resolve-BackupPath $Destination $file.path
    [IO.Directory]::CreateDirectory((Split-Path $target -Parent)) | Out-Null
    [IO.File]::Copy((Resolve-BackupPath $Backup $file.path), $target, $false)
}
[IO.File]::Copy("$Backup/manifest.json", "$Destination/restore-manifest.json", $false)
Write-Output "已恢复到独立目录：$Destination；尚未启动数据库、Hatchet 或 Worker。更换路径后须按恢复手册在副本库重定位工作区。"
