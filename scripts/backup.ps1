param(
    [string]$Root = (Split-Path $PSScriptRoot -Parent),
    [Parameter(Mandatory=$true)][string]$WorkspaceRoot,
    [Parameter(Mandatory=$true)][string]$AuditRoot,
    [Parameter(Mandatory=$true)][string]$Destination
)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/backup-files.ps1"
$Root = [IO.Path]::GetFullPath($Root)
$Destination = [IO.Path]::GetFullPath($Destination)
$databaseRoot = Join-Path $Root '.tools/hatchet-postgres'
if (!(Test-Path -LiteralPath "$databaseRoot/data/PG_VERSION")) { throw '本地 PostgreSQL 数据目录不存在' }
if (Test-Path -LiteralPath "$databaseRoot/data/postmaster.pid") { throw '数据库未确认干净停止；拒绝热复制' }
$rootPrefix = $Root.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
foreach ($process in Get-CimInstance Win32_Process) {
    $panguRunning = $process.Name -match '^pangu.*\.exe$'
    $localDatabase = $process.ExecutablePath -and $process.ExecutablePath.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase) -and $process.Name -match '^(sidecar|postgres)\.exe$'
    if ($panguRunning -or $localDatabase) { throw '请先停止本机所有 Pangu 进程及本实例 Hatchet、PostgreSQL 后备份；go run 的程序也可能在临时目录运行' }
}
$sources = @{'postgres'=$databaseRoot; 'workspaces'=$WorkspaceRoot; 'audit'=$AuditRoot}
foreach ($source in $sources.Values) {
    $prefix = [IO.Path]::GetFullPath($source).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    if ($Destination.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -or $Destination -eq $prefix.TrimEnd('\')) { throw '备份目标不能放在备份源目录内' }
    if (!(Test-Path -LiteralPath $source -PathType Container)) { throw '工作区或审计目录不存在，请传入当前实例的实际目录' }
}
New-PrivateBackupDirectory $Destination
Copy-BackupTree $databaseRoot "$Destination/.tools/hatchet-postgres"
Copy-BackupTree $WorkspaceRoot "$Destination/workspaces"
Copy-BackupTree $AuditRoot "$Destination/audit"
foreach ($name in @('pangu-database.json','hatchet-admin-password')) {
    $source = Join-Path $Root ".tools/$name"
    if (Test-Path -LiteralPath $source) { Copy-BackupTree $source "$Destination/.tools/$name" }
}
$files = @(); $directories = @()
foreach ($item in Get-BackupTree $Destination) {
    $relative = $item.FullName.Substring($Destination.Length).TrimStart('\', '/')
    if ($relative -eq '') { continue }
    if ($item.PSIsContainer) { $directories += $relative }
    else { $files += @{path=$relative; sha256=(Get-FileHash -LiteralPath $item.FullName -Algorithm SHA256).Hash; bytes=$item.Length} }
}
$manifest = @{format='pangu-cold-backup-v1'; created_at=[DateTime]::UtcNow.ToString('o'); postgres_version=[IO.File]::ReadAllText("$databaseRoot/data/PG_VERSION").Trim(); workspace_root=[IO.Path]::GetFullPath($WorkspaceRoot); directories=@($directories); files=@($files)}
if (Test-Path -LiteralPath "$databaseRoot/data/postmaster.pid") { throw '备份期间数据库被启动，本次副本无效' }
[IO.File]::WriteAllText("$Destination/manifest.json", ($manifest | ConvertTo-Json -Depth 5), [Text.UTF8Encoding]::new($false))
Write-Output "备份完成：$Destination；$($files.Count) 个文件；含数据库凭据，请保持目录访问限制。"
