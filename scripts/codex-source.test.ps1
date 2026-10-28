$ErrorActionPreference='Stop'
. "$PSScriptRoot/codex-source.ps1"
$fixture=Join-Path ([IO.Path]::GetTempPath()) ('pangu-codex-source-'+[guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory("$fixture/codex-rs/core/src/tools") | Out-Null
$utf8=[Text.UTF8Encoding]::new($false)
try {
    & git -C $fixture init -q
    & git -C $fixture config core.autocrlf false
    [IO.File]::WriteAllText("$fixture/codex-rs/Cargo.lock","version = `"0.0.0`"`n",$utf8)
    [IO.File]::WriteAllText("$fixture/codex-rs/core/src/tools/registry.rs","original`n",$utf8)
    & git -C $fixture add .
    & git -C $fixture -c user.name=Test -c user.email=test@invalid commit -qm fixture
    [IO.File]::WriteAllText("$fixture/codex-rs/core/src/tools/registry.rs","patched`n",$utf8)
    $patch=Join-Path $fixture '.git/native.patch'
    [IO.File]::WriteAllText($patch,((& git -C $fixture diff) -join "`n")+"`n",$utf8)
    $policy=Join-Path $fixture '.git/policy.rs'
    [IO.File]::WriteAllText($policy,"policy`n",$utf8)
    Copy-Item -LiteralPath $policy -Destination "$fixture/codex-rs/core/src/tools/pangu_policy.rs"
    [IO.File]::WriteAllText("$fixture/codex-rs/Cargo.lock","version = `"0.163.0`"`n",$utf8)
    Assert-CodexSource $fixture $patch $policy
    foreach($path in @('codex-rs/core/src/tools/registry.rs','codex-rs/Cargo.lock','codex-rs/core/src/tools/pangu_policy.rs','codex-rs/injected.rs')){
        $file=Join-Path $fixture $path
        $before=if(Test-Path -LiteralPath $file){[IO.File]::ReadAllText($file)}else{$null}
        [IO.File]::WriteAllText($file,"unexpected`n",$utf8)
        $rejected=$false
        try{Assert-CodexSource $fixture $patch $policy}catch{$rejected=$true}
        if(!$rejected){throw "未拒绝额外源码变更: $path"}
        if($null -eq $before){Remove-Item -LiteralPath $file}else{[IO.File]::WriteAllText($file,$before,$utf8)}
    }
    Assert-CodexSource $fixture $patch $policy
    Write-Output '固定源码、补丁、锁文件及额外文件拒绝测试通过'
} finally {
    $resolved=[IO.Path]::GetFullPath($fixture)
    if(!$resolved.StartsWith([IO.Path]::GetFullPath([IO.Path]::GetTempPath()),[StringComparison]::OrdinalIgnoreCase) -or !(Split-Path $resolved -Leaf).StartsWith('pangu-codex-source-')){throw '拒绝删除非测试目录'}
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
