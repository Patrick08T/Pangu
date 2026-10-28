param([Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$output=Join-Path $Destination 'licenses'
[IO.Directory]::CreateDirectory($output) | Out-Null
Push-Location -LiteralPath "$projectRoot/.tools/codex-runtime/codex-rs"
try {
    $metadata=& cargo +stable metadata --locked --format-version 1 --filter-platform x86_64-pc-windows-msvc
    if($LASTEXITCODE -ne 0){throw '请先下载固定 Cargo 锁文件对应的依赖，不能省略依赖清单'}
    $packages=($metadata | ConvertFrom-Json).packages
    $manifest=@()
    foreach($package in $packages){
        if(!$package.source){continue}
        $directory=Split-Path $package.manifest_path -Parent
        $files=@(Get-ChildItem -LiteralPath $directory -File | Where-Object {$_.Name -match '^(LICENSE|COPYING|NOTICE)'})
        if($package.license_file){$files+=Get-Item -LiteralPath (Join-Path $directory $package.license_file)}
        $hasText=$files.Count -gt 0
        if(!$hasText){$files=@(Get-ChildItem -LiteralPath $directory -File | Where-Object {$_.Name -match '^(README|Cargo.toml.orig)'})}
        foreach($file in $files | Sort-Object FullName -Unique){Copy-Item -LiteralPath $file.FullName -Destination "$output/rust-$($package.name)-$($package.version)-$($file.Name)"}
        $manifest+=@{name=$package.name;version=$package.version;license=$package.license;repository=$package.repository;source=$package.source;license_text_in_crate=$hasText}
    }
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath "$output/Codex-依赖清单.json" -Encoding UTF8
    if(!$manifest.Count){throw 'Cargo 依赖清单为空'}
} finally {Pop-Location}
$entries=& node -e "const p=require(process.argv[1]).packages; console.log(JSON.stringify(Object.keys(p).filter(k=>k&&!p[k].dev)))" "$projectRoot/web/package-lock.json"
if($LASTEXITCODE -ne 0){throw '前端依赖清单读取失败'}
foreach($entry in ($entries | ConvertFrom-Json)){
    $directory=Join-Path "$projectRoot/web" $entry
    foreach($file in Get-ChildItem -LiteralPath $directory -File | Where-Object {$_.Name -match '^(LICENSE|COPYING|NOTICE)'}){
        $name=$entry.Replace('/','_')
        Copy-Item -LiteralPath $file.FullName -Destination "$output/npm-$name-$($file.Name)"
    }
}
