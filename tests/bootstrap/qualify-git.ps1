[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$GitDirectory
)
$ErrorActionPreference='Stop'
$Root=[IO.Path]::GetFullPath($Root)
$GitDirectory=[IO.Path]::GetFullPath($GitDirectory)
if(Test-Path -LiteralPath $Root){throw 'Qualification root must be new'}
New-Item -ItemType Directory -Path $Root | Out-Null
$empty=Join-Path $Root 'empty'
$profile=Join-Path $Root 'private-profile'
$hostile=Join-Path $Root 'hostile'
New-Item -ItemType Directory -Path $empty,$profile,$hostile | Out-Null
$marker=Join-Path $Root 'host-config-executed'
$hostConfig=Join-Path $hostile '.gitconfig'
[IO.File]::WriteAllText($hostConfig,('[core]'+"`n"+'hooksPath = '+$hostile.Replace('\','/')+"`n"+'[credential]'+"`n"+'helper = !echo host-config-executed'+"`n"+'[init]'+"`n"+'templateDir = '+$hostile.Replace('\','/')+"`n"))
[IO.File]::WriteAllText((Join-Path $hostile 'post-checkout'),('#!/bin/sh'+"`n"+'echo bad > "'+$marker.Replace('\','/')+'"'+"`n"))
$before=[IO.File]::ReadAllBytes($hostConfig)
$pathBefore=$env:PATH
$hostSentinels=@{}
foreach($path in @((Join-Path $env:USERPROFILE '.gitconfig'),(Join-Path $env:USERPROFILE '.config\scoop\config.json'),(Join-Path $env:USERPROFILE 'scoop\config.json'))){
    $hostSentinels[$path]=$null
    if(Test-Path -LiteralPath $path -PathType Leaf){$hostSentinels[$path]=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash}
}
$persistentBefore=@{}
foreach($scope in @('User','Machine')){foreach($name in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){$persistentBefore[$scope+'/'+$name]=[Environment]::GetEnvironmentVariable($name,$scope)}}
function Invoke-PrivateGit {
    param([string[]]$Arguments)
    $start=New-Object Diagnostics.ProcessStartInfo
    $start.FileName=Join-Path $GitDirectory 'cmd\git.exe'
    $start.WorkingDirectory=$Root
    $start.UseShellExecute=$false
    $start.CreateNoWindow=$true
    $start.RedirectStandardOutput=$true
    $start.RedirectStandardError=$true
    # Deliberately contaminate the child setup, then apply the proposed boundary.
    $start.EnvironmentVariables['GIT_CONFIG_GLOBAL']=$hostConfig
    $start.EnvironmentVariables['GIT_CONFIG_COUNT']='1'
    $start.EnvironmentVariables['GIT_CONFIG_KEY_0']='core.hooksPath'
    $start.EnvironmentVariables['GIT_CONFIG_VALUE_0']=$hostile
    $start.EnvironmentVariables['GIT_EXEC_PATH']=$hostile
    foreach($key in @($start.EnvironmentVariables.Keys)){
        if([string]$key -like 'GIT_*'){$start.EnvironmentVariables.Remove([string]$key)}
    }
    $start.EnvironmentVariables['GIT_CONFIG_NOSYSTEM']='1'
    $start.EnvironmentVariables['GIT_CONFIG_SYSTEM']='NUL'
    $start.EnvironmentVariables['GIT_CONFIG_GLOBAL']='NUL'
    $start.EnvironmentVariables['GIT_TERMINAL_PROMPT']='0'
    $start.EnvironmentVariables['HOME']=$profile
    $start.EnvironmentVariables['XDG_CONFIG_HOME']=$profile
    $start.EnvironmentVariables['PATH']=Join-Path $env:SystemRoot 'System32'
    # Fixture paths cannot contain quotes; arguments are quoted individually.
    $values=@('-c','credential.helper=','-c',('core.hooksPath='+$empty.Replace('\','/')),'-c',('init.templateDir='+$empty.Replace('\','/')))+$Arguments
    foreach($value in $values){if($value.Contains('"') -or $value.EndsWith('\')){throw 'Unsupported fixture argument'}}
    $start.Arguments=($values | ForEach-Object {'"'+$_+'"'}) -join ' '
    $process=New-Object Diagnostics.Process
    $process.StartInfo=$start
    try {
        if(-not $process.Start()){throw 'Could not start private Git'}
        $stdout=$process.StandardOutput.ReadToEndAsync()
        $stderr=$process.StandardError.ReadToEndAsync()
        if(-not $process.WaitForExit(30000)){$process.Kill();throw 'Private Git timed out'}
        $process.WaitForExit()
        if($process.ExitCode -ne 0){throw ('Private Git failed: '+$stderr.Result)}
        return $stdout.Result.Trim()
    } finally {$process.Dispose()}
}
$version=Invoke-PrivateGit @('--version')
$source=Join-Path $Root 'source'
$clone=Join-Path $Root 'clone'
$null=Invoke-PrivateGit @('init',$source)
[IO.File]::WriteAllText((Join-Path $source 'fixture.txt'),'private Git fixture')
$null=Invoke-PrivateGit @('-C',$source,'add','fixture.txt')
$null=Invoke-PrivateGit @('-C',$source,'-c','user.name=Portscoop fixture','-c','user.email=portscoop@example.invalid','commit','-m','fixture')
$null=Invoke-PrivateGit @('clone','--no-hardlinks',$source,$clone)
if([IO.File]::ReadAllText((Join-Path $clone 'fixture.txt')) -cne 'private Git fixture'){throw 'Clone content mismatch'}
if(Test-Path -LiteralPath $marker){throw 'Hostile Git hook executed'}
if([Convert]::ToBase64String($before) -cne [Convert]::ToBase64String([IO.File]::ReadAllBytes($hostConfig))){throw 'Host fixture config changed'}
if($env:PATH -cne $pathBefore){throw 'Caller PATH changed'}
foreach($path in $hostSentinels.Keys){
    $after=$null
    if(Test-Path -LiteralPath $path -PathType Leaf){$after=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash}
    if($after -cne $hostSentinels[$path]){throw 'Host sentinel changed'}
}
foreach($scope in @('User','Machine')){foreach($name in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){if([Environment]::GetEnvironmentVariable($name,$scope) -cne $persistentBefore[$scope+'/'+$name]){throw 'Persistent environment changed'}}}
@{passed=$true;git_version=$version;private_git=(Join-Path $GitDirectory 'cmd\git.exe');host_sentinels_checked=$hostSentinels.Count} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $Root 'result.json') -Encoding UTF8
