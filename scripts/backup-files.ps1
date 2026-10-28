function Resolve-BackupPath([string]$Root, [string]$Relative) {
    if ([IO.Path]::IsPathRooted($Relative) -or $Relative.Contains(':') -or ($Relative -split '[\\/]') -contains '..') { throw '备份清单包含越界路径' }
    $prefix = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $result = [IO.Path]::GetFullPath((Join-Path $prefix $Relative))
    if (!$result.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { throw '备份清单包含越界路径' }
    return $result
}

function New-PrivateBackupDirectory([string]$Path) {
    if (Test-Path -LiteralPath $Path) { throw '目标目录已存在；恢复和备份均禁止覆盖' }
    [IO.Directory]::CreateDirectory($Path) | Out-Null
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $owner = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $acl.SetOwner($owner)
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($principal in @($owner, [Security.Principal.SecurityIdentifier]::new('S-1-5-18'))) {
        $rule = [Security.AccessControl.FileSystemAccessRule]::new($principal, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}

function Get-BackupTree([string]$Root) {
    $pending = New-Object 'Collections.Generic.Stack[string]'
    $pending.Push([IO.Path]::GetFullPath($Root))
    while ($pending.Count -gt 0) {
        $path = $pending.Pop()
        $item = Get-Item -LiteralPath $path -Force
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw '备份拒绝符号链接和 junction，避免越出托管目录' }
        $item
        if ($item.PSIsContainer) {
            foreach ($child in Get-ChildItem -LiteralPath $path -Force) { $pending.Push($child.FullName) }
        }
    }
}

function Copy-BackupTree([string]$Source, [string]$Target) {
    $base = [IO.Path]::GetFullPath($Source).TrimEnd('\', '/')
    foreach ($item in Get-BackupTree $base) {
        $relative = $item.FullName.Substring($base.Length).TrimStart('\', '/')
        $destination = if ($relative -eq '') { $Target } else { Resolve-BackupPath $Target $relative }
        if ($item.PSIsContainer) { [IO.Directory]::CreateDirectory($destination) | Out-Null }
        else {
            [IO.Directory]::CreateDirectory((Split-Path $destination -Parent)) | Out-Null
            [IO.File]::Copy($item.FullName, $destination, $false)
        }
    }
}
