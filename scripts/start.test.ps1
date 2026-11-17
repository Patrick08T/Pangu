$ErrorActionPreference='Stop'
$testRoot=Join-Path ([IO.Path]::GetTempPath()) ('pangu-start-'+[guid]::NewGuid().ToString('N'))
$project=Join-Path $testRoot ([string][char]0x9879+[char]0x76ee+' with spaces')
$localData=Join-Path $testRoot 'user data'
$saved=@{}
$names=@('LOCALAPPDATA','PANGU_WORKSPACE_ROOT','PANGU_AUDIT_SPOOL_ROOT','PANGU_CODEX_HOME','PANGU_DATABASE_URL','PANGU_DATABASE_ADMIN_URL')
foreach($name in $names){$saved[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
$previousHarness=Get-Variable -Name PanguStartTestState -Scope Global -ValueOnly -ErrorAction SilentlyContinue
$global:PanguStartTestState=@{Launch=$null;Stopped=$false;Fake=$null}
function Get-NetTCPConnection { param($State,$LocalPort,$ErrorAction) return $null }
function Start-Sleep { param($Milliseconds) }
function Start-Process {
    param($FilePath,$ArgumentList,$WorkingDirectory,$WindowStyle,[switch]$PassThru,$RedirectStandardOutput,$RedirectStandardError)
    $global:PanguStartTestState.Launch=[IO.Path]::GetFullPath($FilePath)
    $global:PanguStartTestState.Fake=[pscustomobject]@{ProcessId=44123;Id=44123;ExecutablePath=$global:PanguStartTestState.Launch;CreationDate=[datetime]'2026-11-11T10:00:00Z';HasExited=$false}
    return $global:PanguStartTestState.Fake
}
function Get-CimInstance { param($ClassName,$Filter) return $global:PanguStartTestState.Fake }
function Stop-Process { param($Id,$ErrorAction) if($Id -ne 44123){throw 'Unexpected process'}; $global:PanguStartTestState.Stopped=$true }
function Wait-Process { param($Id,$Timeout,$ErrorAction) }
function Clear-TestRecord {
    $record=Join-Path $project '.tools/pangu-process.json'
    if((Split-Path $record -Parent) -ne (Join-Path $project '.tools')){throw 'Unexpected test record'}
    if(Test-Path -LiteralPath $record){Remove-Item -LiteralPath $record}
    $global:PanguStartTestState.Fake=$null
}
function Assert-Launch([string]$Expected,[switch]$Installed) {
    Clear-TestRecord
    $global:PanguStartTestState.Launch=$null
    & "$project/scripts/start.ps1" -Mode all -Installed:$Installed | Out-Null
    if($global:PanguStartTestState.Launch -ne [IO.Path]::GetFullPath($Expected)){throw "Wrong binary: expected $Expected; got $($global:PanguStartTestState.Launch)"}
    $global:PanguStartTestState.Stopped=$false
    & "$project/scripts/stop.ps1" | Out-Null
    if(!$global:PanguStartTestState.Stopped -or (Test-Path -LiteralPath "$project/.tools/pangu-process.json")){throw 'Stop did not handle registered process'}
    $global:PanguStartTestState.Fake=$null
}
try {
    foreach($directory in @("$project/scripts","$project/.tools","$localData/Pangu/bin")){[IO.Directory]::CreateDirectory($directory)|Out-Null}
    foreach($file in @('start.ps1','stop.ps1')){Copy-Item -LiteralPath "$PSScriptRoot/$file" -Destination "$project/scripts/$file"}
    foreach($file in @('hatchet-env.ps1','database-env.ps1')){[IO.File]::WriteAllText("$project/scripts/$file",'')}
    [IO.File]::WriteAllText("$project/.tools/pangu.exe",'bundled-fixture')
    $installed=Join-Path $localData 'Pangu/bin/pangu.exe'
    [IO.File]::WriteAllText($installed,'installed-fixture')
    $env:LOCALAPPDATA=$localData
    $env:PANGU_WORKSPACE_ROOT=Join-Path $project 'workspaces'
    $env:PANGU_AUDIT_SPOOL_ROOT=Join-Path $project 'audit'
    $env:PANGU_CODEX_HOME=Join-Path $project 'codex'
    $env:PANGU_DATABASE_URL='fixture-restored'
    $env:PANGU_DATABASE_ADMIN_URL='fixture-not-inherited'
    Assert-Launch $installed -Installed
    Assert-Launch "$project/.tools/pangu.exe"
    [IO.File]::WriteAllText("$project/pangu.exe",'root-fixture')
    Assert-Launch "$project/pangu.exe"
    Assert-Launch $installed -Installed
    if($installed -ne (Join-Path $localData 'Pangu/bin/pangu.exe')){throw 'Unexpected installed fixture'}
    Remove-Item -LiteralPath $installed
    $global:PanguStartTestState.Launch=$null
    $rejected=$false
    try { & "$project/scripts/start.ps1" -Installed | Out-Null } catch { $rejected=$_.Exception.Message -match 'pangu.exe' }
    if(!$rejected -or $global:PanguStartTestState.Launch){throw 'Missing installed binary must not fall back or launch'}
    if($env:PANGU_DATABASE_URL -ne 'fixture-restored' -or $env:PANGU_DATABASE_ADMIN_URL -ne 'fixture-not-inherited'){throw 'Caller environment changed'}
    Write-Output 'Startup tests passed: explicit installed path, Unicode/spaces, registered stop, legacy root/tools selection, no missing-binary fallback, caller environment preserved.'
} finally {
    if($null -eq $previousHarness){Remove-Variable -Name PanguStartTestState -Scope Global}else{$global:PanguStartTestState=$previousHarness}
    foreach($name in $names){[Environment]::SetEnvironmentVariable($name,$saved[$name],'Process')}
    $resolved=[IO.Path]::GetFullPath($testRoot)
    $temporary=[IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\','/')+[IO.Path]::DirectorySeparatorChar
    if(!$resolved.StartsWith($temporary,[StringComparison]::OrdinalIgnoreCase) -or !(Split-Path $resolved -Leaf).StartsWith('pangu-start-')){throw 'Refusing non-fixture cleanup'}
    if(Test-Path -LiteralPath $resolved){Remove-Item -LiteralPath $resolved -Recurse -Force}
}
