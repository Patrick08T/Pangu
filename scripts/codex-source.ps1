function Assert-CodexSource([string]$Source,[string]$Patch,[string]$Policy) {
    $previousIndex=$env:GIT_INDEX_FILE
    $index=Join-Path ([IO.Path]::GetTempPath()) ('pangu-index-'+[guid]::NewGuid().ToString('N'))
    $lockFile=$index+'.lock-content'
    try {
        $env:GIT_INDEX_FILE=$index
        & git -C $Source read-tree HEAD
        if($LASTEXITCODE -ne 0){throw '读取固定源码失败'}
        & git -C $Source apply --cached $Patch
        if($LASTEXITCODE -ne 0){throw '固定补丁与上游不匹配'}
        $lock=& git -C $Source show HEAD:codex-rs/Cargo.lock
        if($LASTEXITCODE -ne 0){throw '读取原始锁文件失败'}
        $content=($lock -join "`n").Replace('version = "0.0.0"','version = "0.163.0"')+"`n"
        [IO.File]::WriteAllText($lockFile,$content,[Text.UTF8Encoding]::new($false))
        foreach($entry in @(@('codex-rs/Cargo.lock',$lockFile),@('codex-rs/core/src/tools/pangu_policy.rs',$Policy))){
            $hash=& git -C $Source hash-object -w "--path=$($entry[0])" $entry[1]
            if($LASTEXITCODE -ne 0){throw '固定文件摘要生成失败'}
            & git -C $Source update-index --add --cacheinfo "100644,$hash,$($entry[0])"
            if($LASTEXITCODE -ne 0){throw '固定源码索引生成失败'}
        }
        & git -C $Source diff --no-ext-diff --quiet
        if($LASTEXITCODE -ne 0){throw 'Codex 存在固定补丁以外的源码或锁文件变更，拒绝构建'}
        $extra=& git -C $Source ls-files --others --exclude-standard
        if($LASTEXITCODE -ne 0 -or $extra){throw 'Codex 存在额外未跟踪文件，拒绝构建'}
    } finally {
        $env:GIT_INDEX_FILE=$previousIndex
        foreach($file in @($index,$lockFile)){if(Test-Path -LiteralPath $file){Remove-Item -LiteralPath $file -Force}}
    }
}
