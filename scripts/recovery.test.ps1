$ErrorActionPreference = 'Stop'
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('pangu-backup-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path "$testRoot/source/.tools/hatchet-postgres/data", "$testRoot/workspaces/run", "$testRoot/audit" -Force | Out-Null
[IO.File]::WriteAllText("$testRoot/source/.tools/hatchet-postgres/data/PG_VERSION", '18')
[IO.File]::WriteAllText("$testRoot/source/.tools/pangu-database.json", '{"database":"test-only"}')
[IO.File]::WriteAllText("$testRoot/workspaces/run/中文.txt", '恢复证据')
function Assert-Rejected([scriptblock]$Action) {
    $rejected = $false
    try { & $Action | Out-Null } catch { $rejected = $true }
    if (!$rejected) { throw '预期拒绝的恢复操作被接受' }
}
try {
    [IO.File]::WriteAllText("$testRoot/source/.tools/hatchet-postgres/data/postmaster.pid", '123')
    Assert-Rejected { & "$PSScriptRoot/backup.ps1" -Root "$testRoot/source" -WorkspaceRoot "$testRoot/workspaces" -AuditRoot "$testRoot/audit" -Destination "$testRoot/active" }
    Remove-Item -LiteralPath "$testRoot/source/.tools/hatchet-postgres/data/postmaster.pid"
    Copy-Item -LiteralPath "$env:SystemRoot/System32/ping.exe" -Destination "$testRoot/pangu-backup-helper.exe"
    $helper = Start-Process -FilePath "$testRoot/pangu-backup-helper.exe" -ArgumentList @('-t','127.0.0.1') -WindowStyle Hidden -PassThru
    try {
        Assert-Rejected { & "$PSScriptRoot/backup.ps1" -Root "$testRoot/source" -WorkspaceRoot "$testRoot/workspaces" -AuditRoot "$testRoot/audit" -Destination "$testRoot/external-worker" }
    } finally { Stop-Process -Id $helper.Id; $helper.WaitForExit() }
    & "$PSScriptRoot/backup.ps1" -Root "$testRoot/source" -WorkspaceRoot "$testRoot/workspaces" -AuditRoot "$testRoot/audit" -Destination "$testRoot/backup"
    & "$PSScriptRoot/restore.ps1" -Backup "$testRoot/backup" -Destination "$testRoot/restored"
    if ([IO.File]::ReadAllText("$testRoot/restored/workspaces/run/中文.txt") -ne '恢复证据') { throw '文件未恢复' }
    Assert-Rejected { & "$PSScriptRoot/restore.ps1" -Backup "$testRoot/backup" -Destination "$testRoot/restored" }
    $manifestPath = "$testRoot/backup/manifest.json"
    $originalManifest = [IO.File]::ReadAllText($manifestPath)
    $manifest = $originalManifest | ConvertFrom-Json
    $manifest.directories = @('../outside')
    [IO.File]::WriteAllText($manifestPath,($manifest | ConvertTo-Json -Depth 5))
    Assert-Rejected { & "$PSScriptRoot/restore.ps1" -Backup "$testRoot/backup" -Destination "$testRoot/escaped" }
    [IO.File]::WriteAllText($manifestPath,$originalManifest)
    [IO.File]::WriteAllText("$testRoot/backup/workspaces/run/中文.txt", '被改写')
    Assert-Rejected { & "$PSScriptRoot/restore.ps1" -Backup "$testRoot/backup" -Destination "$testRoot/tampered" }
    Write-Output '备份/恢复脚本测试通过：拒绝活动库、禁止覆盖、中文文件、损坏检测。'
} finally {
    $resolved = [IO.Path]::GetFullPath($testRoot)
    $temporary = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if (!$resolved.StartsWith($temporary, [StringComparison]::OrdinalIgnoreCase) -or !(Split-Path $resolved -Leaf).StartsWith('pangu-backup-')) { throw '拒绝删除非测试目录' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
