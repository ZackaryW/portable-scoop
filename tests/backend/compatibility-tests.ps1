#Requires -Version 5.1
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Root, [string]$Backend)
$ErrorActionPreference = 'Stop'
$Root = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Root)
if ($Backend) { $Backend = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Backend) }
$project = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $project 'backend\scoop\adapter\system.ps1')
. (Join-Path $project 'backend\scoop\adapter\patch-functions.ps1')
function Assert([bool]$Condition, [string]$Message) { if (-not $Condition) { throw $Message } }
function Reject([scriptblock]$Body) {
    $rejected = $false
    try { & $Body } catch { $rejected = $true }
    Assert $rejected 'Expected action rejection'
}
# Test doubles provide only declaration lookup; no upstream script evaluation.
function arch_specific($name, $manifest, $arch) { return $manifest.$name }
$env:PORTSCOOP_ROOT = [IO.Path]::GetFullPath($Root)
$app = 'fixture'
$dir = Join-Path $Root 'scoop\apps\fixture\1.0'
$persist_dir = Join-Path $Root 'scoop\persist\fixture'
New-Item -ItemType Directory -Path $dir,$persist_dir -Force | Out-Null
$action = [pscustomobject]@{phase='pre_install';kind='ensure-persisted-file';path='config.xml'}
$manifest = [pscustomobject]@{version='1.0';pre_install='this string must never be evaluated'}
$portscoopRequest = [pscustomobject]@{manifests=[pscustomobject]@{fixture=[pscustomobject]@{manifest=$manifest;actions=@($action)}}}
Invoke-HookScript pre_install $manifest 64bit
Assert ([IO.File]::Exists("$dir\config.xml")) 'Configuration was not created'
Assert ((Get-Item -LiteralPath "$dir\config.xml").Length -eq 0) 'Configuration was not empty'
[IO.File]::WriteAllText("$persist_dir\config.xml",'saved configuration')
[IO.File]::WriteAllText("$dir\config.xml",'existing package configuration')
Invoke-HookScript pre_install $manifest 64bit
Assert ([IO.File]::ReadAllText("$persist_dir\config.xml") -ceq 'saved configuration') 'Persistence was overwritten'
Assert ([IO.File]::ReadAllText("$dir\config.xml") -ceq 'existing package configuration') 'Package file was overwritten'

Reject { Invoke-PortscoopAction $action $app '1.0' (Join-Path $Root 'elsewhere') $persist_dir }
$action.path = '..\escape.xml'
Reject { Invoke-HookScript pre_install $manifest 64bit }
$action.path = 'config.xml'; $action.kind = 'eval'
Reject { Invoke-HookScript pre_install $manifest 64bit }
$action.kind = 'ensure-persisted-file'
$portscoopRequest.manifests.fixture.actions = @()
Reject { Invoke-HookScript pre_install $manifest 64bit }
$portscoopRequest.manifests.fixture.actions = @($action)
Reject { Invoke-HookScript pre_install ([pscustomobject]@{version='1.0';pre_install='different'}) 64bit }
Reject { Invoke-HookScript post_install ([pscustomobject]@{post_install='not evaluated'}) 64bit }
Reject { install_psmodule ([pscustomobject]@{psmodule=@{name='Test'}}) $dir $false }
Reject { create_startmenu_shortcuts ([pscustomobject]@{shortcuts=@(,@('app.exe','App'))}) $dir $false 64bit }

$action.path = 'directory.xml'
New-Item -ItemType Directory -Path "$persist_dir\directory.xml" | Out-Null
Reject { Invoke-HookScript pre_install $manifest 64bit }
$action.path = 'config.xml'
# Junctions need no symlink privilege. All targets are inside this test root.
$outside = Join-Path $Root 'sentinel'
New-Item -ItemType Directory -Path $outside | Out-Null
[IO.File]::WriteAllText("$outside\config.xml",'sentinel')
$linkVersion = Join-Path $Root 'scoop\apps\fixture\2.0'
New-Item -ItemType Junction -Path $linkVersion -Target $outside | Out-Null
Reject { Invoke-PortscoopAction $action $app '2.0' $linkVersion $persist_dir }
Assert ([IO.File]::ReadAllText("$outside\config.xml") -ceq 'sentinel') 'Reparse target changed'
[IO.Directory]::Delete($linkVersion)
# Missing persistence is normal; no persistence directory is created by the hook.
Invoke-PortscoopAction $action 'newapp' '1.0' (New-Item -ItemType Directory -Path "$Root\scoop\apps\newapp\1.0" -Force).FullName "$Root\scoop\persist\newapp"
Assert ([IO.File]::Exists("$Root\scoop\apps\newapp\1.0\config.xml")) 'Missing persistence failed'
if ($Backend) {
    # Load only the pinned backend's hash functions, without its module startup
    # or any package hooks. The input is a prepared, verified build artifact.
    $source = Join-Path $Backend 'engine\lib\download.ps1'
    $tokens = $null; $parseErrors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
    Assert ($parseErrors.Count -eq 0) 'Invalid prepared download source'
    $functions = @($ast.FindAll({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -in @('check_hash','get_hash')}, $false))
    Assert ($functions.Count -eq 2) 'Missing prepared hash functions'
    foreach ($function in $functions) { . ([scriptblock]::Create($function.Extent.Text)) }
    function url_remote_filename($url) { return 'fixture.zip' }
    function get_magic_bytes_pretty($file, $separator) { return 'fixture' }
    $payload = Join-Path $Root 'digest-fixture.zip'
    [IO.File]::WriteAllText($payload,'controlled fixture contents')
    $url = 'https://example.invalid/fixture.zip'
    foreach ($algorithm in @('SHA1','SHA256','SHA512','MD5')) {
        $digest = (Get-FileHash -LiteralPath $payload -Algorithm $algorithm).Hash.ToLowerInvariant()
        $ok, $errorText = check_hash $payload ($algorithm.ToLowerInvariant()+':'+$digest) 'fixture'
        Assert $ok "Valid $algorithm digest rejected: $errorText"
        $ok, $errorText = check_hash $payload ($algorithm.ToLowerInvariant()+':'+('0'*$digest.Length)) 'fixture'
        Assert (-not $ok) "Mismatched $algorithm digest accepted"
    }
}
Write-Output 'Portable action checks passed'
