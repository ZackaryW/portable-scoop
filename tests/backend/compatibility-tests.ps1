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
$action = [pscustomobject]@{phase='pre_install';kind='ensure-persisted-file';path='config.xml';persist_path='config.xml'}
$manifest = [pscustomobject]@{version='1.0';pre_install='this string must never be evaluated';persist='config.xml'}
$portscoopRequest = [pscustomobject]@{manifests=[pscustomobject]@{fixture=[pscustomobject]@{manifest=$manifest;actions=@($action);decisions=@([pscustomobject]@{field='pre_install';disposition='rewrite'})}}}
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
$action.persist_path = 'directory.xml'
New-Item -ItemType Directory -Path "$persist_dir\directory.xml" | Out-Null
Reject { Invoke-HookScript pre_install $manifest 64bit }
$action.path = 'config.xml'
$action.persist_path = 'config.xml'
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

# Ordered rename chains, missing sources, and non-overwriting destinations.
$first=[pscustomobject]@{phase='pre_install';kind='rename-package-file';source='first.exe';path='second.exe'}
$second=[pscustomobject]@{phase='pre_install';kind='rename-package-file';source='second.exe';path='third.exe'}
[IO.File]::WriteAllText("$dir\first.exe",'inert executable fixture')
$portscoopRequest.manifests.fixture.actions=@($first,$second)
Invoke-HookScript pre_install $manifest 64bit
Assert ([IO.File]::ReadAllText("$dir\third.exe") -ceq 'inert executable fixture') 'Rename ordering failed'
Reject { Invoke-HookScript pre_install $manifest 64bit }
[IO.File]::WriteAllText("$dir\first.exe",'preserve source')
[IO.File]::WriteAllText("$dir\second.exe",'preserve destination')
Reject { Invoke-HookScript pre_install $manifest 64bit }
Assert ([IO.File]::ReadAllText("$dir\second.exe") -ceq 'preserve destination') 'Rename overwrote destination'

# An explicitly skipped hook permits no actions and never evaluates its text.
$portscoopRequest.manifests.fixture.actions=@()
$portscoopRequest.manifests.fixture.decisions[0].disposition='skip'
Invoke-HookScript pre_install $manifest 64bit

# Post-install runs through the exact current junction and an owned hard link.
$physical=$dir
$dir=Join-Path $Root 'scoop\apps\fixture\current'
New-Item -ItemType Junction -Path $dir -Target $physical | Out-Null
$first.phase='post_install'; $first.source='third.exe'; $first.path='final.exe'
Invoke-PortscoopAction $first $app '1.0' $dir $persist_dir
Assert ([IO.File]::Exists("$physical\final.exe")) 'Post-install rename failed'
Reject { Invoke-PortscoopAction $first $app '1.0' $physical $persist_dir }
[IO.File]::Delete("$physical\config.xml")
New-Item -ItemType HardLink -Path "$physical\config.xml" -Target "$persist_dir\config.xml" | Out-Null
$action.phase='post_install'
Invoke-PortscoopAction $action $app '1.0' $dir $persist_dir
Assert ([IO.File]::ReadAllText("$persist_dir\config.xml") -ceq 'saved configuration') 'Post-install changed persistence'
[IO.File]::Delete("$physical\config.xml")
[IO.File]::WriteAllText("$physical\config.xml",'unrelated equal-looking file')
Reject { Invoke-PortscoopAction $action $app '1.0' $dir $persist_dir }
[IO.Directory]::Delete($dir)
New-Item -ItemType Junction -Path $dir -Target $outside | Out-Null
Reject { Invoke-PortscoopAction $action $app '1.0' $dir $persist_dir }
[IO.Directory]::Delete($dir)
$dir=$physical

# Generic packaged defaults and literal initialization in both layouts.
$directory=[pscustomobject]@{phase='pre_install';kind='ensure-directory';path='cache/nested'}
Invoke-PortscoopAction $directory $app '1.0' $physical $persist_dir
Assert ([IO.Directory]::Exists("$physical\cache\nested")) 'Local directory not created'
Invoke-PortscoopAction $directory $app '1.0' $physical $persist_dir
$local=[pscustomobject]@{phase='pre_install';kind='copy-package-file';source='defaults.ini';path='profile/options.ini';persist_path='saved/options.ini';guard='persist-missing'}
[IO.File]::WriteAllText("$physical\defaults.ini",'packaged default')
Invoke-PortscoopAction $local $app '1.0' $physical $persist_dir
Assert ([IO.File]::ReadAllText("$physical\profile\options.ini") -ceq 'packaged default') 'Packaged default not copied'
New-Item -ItemType Directory -Path "$persist_dir\saved" -Force | Out-Null
[IO.File]::WriteAllText("$persist_dir\saved\options.ini",'saved default')
$local.source='missing.ini'
Invoke-PortscoopAction $local $app '1.0' $physical $persist_dir
Assert ([IO.File]::ReadAllText("$persist_dir\saved\options.ini") -ceq 'saved default') 'Saved default overwritten'
$literal=[pscustomobject]@{phase='pre_install';kind='write-default-file';path='literal.ini';content="hello`r`n";encoding='ascii'}
Invoke-PortscoopAction $literal $app '1.0' $physical $persist_dir
Assert ([IO.File]::ReadAllText("$physical\literal.ini") -ceq "hello`r`n") 'Literal bytes differ'
# Replacing a package entry must not truncate other names for the same file.
[IO.File]::WriteAllText("$outside\linked.ini",'untouched')
New-Item -ItemType HardLink -Path "$physical\linked.ini" -Target "$outside\linked.ini" | Out-Null
$literal.path='linked.ini'
Invoke-PortscoopAction $literal $app '1.0' $physical $persist_dir
Assert ([IO.File]::ReadAllText("$outside\linked.ini") -ceq 'untouched') 'Literal followed a hard link'

# A post-install default inside a declared persisted directory uses its checked
# physical saved location, never an arbitrary junction.
[IO.File]::Delete("$physical\profile\options.ini")
[IO.Directory]::Delete("$physical\profile")
New-Item -ItemType Junction -Path "$physical\profile" -Target "$persist_dir\saved" | Out-Null
$current=Join-Path $Root 'scoop\apps\fixture\current'
New-Item -ItemType Junction -Path $current -Target $physical | Out-Null
$literal.phase='post_install';$literal.path='profile/new.ini'
$literal | Add-Member persist_path 'saved/new.ini'
$mapping=@(,@('profile','saved'))
Invoke-PortscoopAction $literal $app '1.0' $current $persist_dir $mapping
Assert ([IO.File]::ReadAllText("$persist_dir\saved\new.ini") -ceq "hello`r`n") 'Post-install default not stored'
$literal.content='do not overwrite'
Invoke-PortscoopAction $literal $app '1.0' $current $persist_dir $mapping
Assert ([IO.File]::ReadAllText("$persist_dir\saved\new.ini") -ceq "hello`r`n") 'Post-install default overwritten'
[IO.Directory]::Delete("$physical\profile")
New-Item -ItemType Junction -Path "$physical\profile" -Target $outside | Out-Null
Reject { Invoke-PortscoopAction $literal $app '1.0' $current $persist_dir $mapping }
[IO.Directory]::Delete("$physical\profile")
[IO.Directory]::Delete($current)

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
