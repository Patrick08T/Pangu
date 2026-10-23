param([Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$Destination=[IO.Path]::GetFullPath($Destination)
if(Test-Path -LiteralPath $Destination){throw '安装包目标已存在，拒绝覆盖'}
if([IO.Path]::GetExtension($Destination) -ne '.zip'){throw '安装包目标必须以 .zip 结尾'}
$goBinary=if(Test-Path "$projectRoot/.tools/go/bin/go.exe"){"$projectRoot/.tools/go/bin/go.exe"}else{(Get-Command go -ErrorAction Stop).Source}
$staging=Join-Path ([IO.Path]::GetTempPath()) ('pangu-package-'+[guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory("$staging/.tools") | Out-Null
[IO.Directory]::CreateDirectory("$staging/licenses") | Out-Null
Push-Location -LiteralPath $projectRoot
try {
    & npm.cmd --prefix web run build
    if($LASTEXITCODE -ne 0){throw '前端构建失败'}
    & $goBinary build -trimpath -o "$staging/pangu.exe" ./cmd/pangu
    if($LASTEXITCODE -ne 0){throw 'Pangu 构建失败'}
    & $goBinary build -trimpath -o "$staging/database.exe" ./cmd/database
    if($LASTEXITCODE -ne 0){throw '建库程序构建失败'}
    if(!(Test-Path '.tools/sidecar.exe')){throw '请先按 hatchet.ps1 构建固定版本 Hatchet sidecar'}
    Copy-Item -LiteralPath '.tools/sidecar.exe' -Destination "$staging/.tools/sidecar.exe"
    foreach($directory in @('docs','examples','api','specs','runtime')){Copy-Item -LiteralPath $directory -Destination $staging -Recurse}
    Copy-Item -LiteralPath 'README.md','.env.example','LICENSE' -Destination $staging
    if(!(Test-Path '.tools/codex/build.json')){throw '请先构建 Codex 受控运行时'}
    $codexBuild=Get-Content '.tools/codex/build.json' -Raw -Encoding UTF8 | ConvertFrom-Json
    foreach($entry in @(@('patch','runtime/codex/native.patch'),@('policy','runtime/codex/policy.rs'),@('binary','.tools/codex/codex.exe'))){
        if((Get-FileHash -LiteralPath $entry[1] -Algorithm SHA256).Hash -ne $codexBuild.($entry[0])){throw 'Codex 构建已过时或被修改，请重新构建'}
    }
    [IO.Directory]::CreateDirectory("$staging/.tools/codex") | Out-Null
    Copy-Item -LiteralPath '.tools/codex/codex.exe' -Destination "$staging/.tools/codex/codex.exe"
    Copy-Item -LiteralPath '.tools/codex/build.json' -Destination "$staging/.tools/codex/build.json"
    Copy-Item -LiteralPath '.tools/codex-runtime/LICENSE' -Destination "$staging/licenses/Codex-LICENSE"
    Copy-Item -LiteralPath '.tools/codex-runtime/NOTICE' -Destination "$staging/licenses/Codex-NOTICE"
    & "$PSScriptRoot/package-licenses.ps1" -Destination $staging
    [IO.Directory]::CreateDirectory("$staging/scripts") | Out-Null
    foreach($name in @('start','stop','hatchet','hatchet-stop','hatchet-env','database-env','database-init','backup','backup-files','restore','codex-build','codex-source')) {
        Copy-Item -LiteralPath "scripts/$name.ps1" -Destination "$staging/scripts"
    }
    $modules=& $goBinary list -m -f '{{.Path}}|{{.Dir}}' all
    if($LASTEXITCODE -ne 0){throw '依赖清单生成失败'}
    $moduleCache=& $goBinary env GOMODCACHE
    $modules+=@("github.com/hatchet-dev/hatchet-embedded|$moduleCache/github.com/hatchet-dev/hatchet-embedded@v0.110.5")
    foreach($module in $modules){
        $parts=$module.Split('|',2)
        if($parts.Count -ne 2 -or !$parts[1]){continue}
        $name=$parts[0].Replace('/','_')
        foreach($license in Get-ChildItem -LiteralPath $parts[1] -File | Where-Object {$_.Name -match '^(LICENSE|COPYING|NOTICE)'}){
            Copy-Item -LiteralPath $license.FullName -Destination "$staging/licenses/$name-$($license.Name)"
        }
    }
    & $goBinary version -m "$staging/pangu.exe" "$staging/database.exe" "$staging/.tools/sidecar.exe" | Set-Content "$staging/构建依赖.txt" -Encoding UTF8
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Destination)) | Out-Null
    [IO.Compression.ZipFile]::CreateFromDirectory($staging,$Destination)
    $archive=[IO.Compression.ZipFile]::OpenRead($Destination)
    try {
        $names=@($archive.Entries | ForEach-Object {$_.FullName.Replace('\','/')})
        foreach($required in @('pangu.exe','database.exe','.tools/sidecar.exe','.tools/codex/codex.exe','LICENSE','scripts/start.ps1','scripts/restore.ps1','runtime/codex/native.patch')){if($names -notcontains $required){throw "安装包缺失 $required"}}
        if($names | Where-Object {$_ -match '(^|/)(auth\.json|\.env|pangu.*database\.json|hatchet-connection\.json|postmaster\.pid)$'}){throw '安装包包含禁止分发的本机配置'}
    } finally {$archive.Dispose()}
    Get-FileHash -LiteralPath $Destination -Algorithm SHA256 | Format-List Path,Hash
} finally {
    Pop-Location
    $resolved=[IO.Path]::GetFullPath($staging)
    $tempRoot=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if(!$resolved.StartsWith($tempRoot,[StringComparison]::OrdinalIgnoreCase) -or !(Split-Path $resolved -Leaf).StartsWith('pangu-package-')){throw '拒绝删除非构建目录'}
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
