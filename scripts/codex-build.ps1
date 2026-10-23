param()
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$source=Join-Path $projectRoot '.tools/codex-runtime'
$revision='83086188ae0614bd882818ff1aa23f3194962012'
$patch=Join-Path $projectRoot 'runtime/codex/native.patch'
if(!(Test-Path -LiteralPath $source)){
    & git clone --depth 1 --branch rust-v0.163.0 https://github.com/openai/codex.git $source
    if($LASTEXITCODE -ne 0){throw 'Codex 源码下载失败'}
}
$head=& git -C $source rev-parse HEAD
if($LASTEXITCODE -ne 0 -or $head -ne $revision){throw 'Codex 源码版本不匹配，拒绝构建'}
$applied=[IO.File]::ReadAllText("$source/codex-rs/core/src/tools/registry.rs").Contains('let _pangu_budget =')
if($applied){
    & git -C $source apply --reverse --check $patch
    if($LASTEXITCODE -ne 0){throw '现有补丁不完整，拒绝构建'}
}else{
    & git -C $source apply --check $patch
    if($LASTEXITCODE -ne 0){throw 'Codex 补丁与源码不匹配'}
    & git -C $source apply $patch
    if($LASTEXITCODE -ne 0){throw '应用 Codex 补丁失败'}
}
Copy-Item -LiteralPath "$projectRoot/runtime/codex/policy.rs" -Destination "$source/codex-rs/core/src/tools/pangu_policy.rs"
# 上游发布标签更新了 workspace 版本，锁文件仍为 0.0.0；仅同步本地包版本。
$lockPath="$source/codex-rs/Cargo.lock"
$lock=[IO.File]::ReadAllText($lockPath).Replace('version = "0.0.0"','version = "0.163.0"')
[IO.File]::WriteAllText($lockPath,$lock,[Text.UTF8Encoding]::new($false))
. "$PSScriptRoot/codex-source.ps1"
Assert-CodexSource $source $patch "$projectRoot/runtime/codex/policy.rs"
& rustc +stable --edition 2024 --test "$projectRoot/runtime/codex/policy.rs" -o "$projectRoot/.tools/codex-policy-test.exe"
if($LASTEXITCODE -ne 0){throw '工具限制测试编译失败'}
& "$projectRoot/.tools/codex-policy-test.exe"
if($LASTEXITCODE -ne 0){throw '工具限制测试失败'}
Push-Location -LiteralPath "$source/codex-rs"
try {
    & cargo +stable build -p codex-cli --bin codex --release --locked -j 4
    if($LASTEXITCODE -ne 0){throw 'Codex 受控构建失败'}
} finally {Pop-Location}
[IO.Directory]::CreateDirectory("$projectRoot/.tools/codex") | Out-Null
Copy-Item -LiteralPath "$source/codex-rs/target/release/codex.exe" -Destination "$projectRoot/.tools/codex/codex.exe"
$manifest=@{
    revision=$revision
    patch=(Get-FileHash -LiteralPath $patch -Algorithm SHA256).Hash
    policy=(Get-FileHash -LiteralPath "$projectRoot/runtime/codex/policy.rs" -Algorithm SHA256).Hash
    binary=(Get-FileHash -LiteralPath "$projectRoot/.tools/codex/codex.exe" -Algorithm SHA256).Hash
    compiler=(& rustc +stable --version)
}
$manifest | ConvertTo-Json | Set-Content -LiteralPath "$projectRoot/.tools/codex/build.json" -Encoding UTF8
Get-FileHash -LiteralPath "$projectRoot/.tools/codex/codex.exe" -Algorithm SHA256
