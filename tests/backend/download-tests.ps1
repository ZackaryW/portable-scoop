#Requires -Version 5.1
param([string]$Root,[string]$Backend,[string]$Address,[string]$Digest)
$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSHOME 'Modules\Microsoft.PowerShell.Utility\Microsoft.PowerShell.Utility.psd1')
$project=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $project 'backend\scoop\adapter\system.ps1')
$tokens=$null; $errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $Backend 'engine\lib\download.ps1'),[ref]$tokens,[ref]$errors)
if ($errors.Count) { throw 'Invalid prepared source' }
$wanted=@('Invoke-ScoopDownload','Invoke-CachedDownload','Start-Download','Invoke-Download','check_hash','get_hash')
foreach ($f in $ast.FindAll({param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -in $wanted},$false)) { . ([scriptblock]::Create($f.Extent.Text)) }
# Isolate upstream configuration and formatting helpers; use its real network,
# cache, and digest implementations. No installed Scoop modules are loaded.
function cache_path($a,$v,$u) { Join-Path $cachedir ([IO.Path]::GetFileName($u)) }
function ensure($p) { New-Item -ItemType Directory -Path $p -Force }
function handle_special_urls($u) { $u }
function Get-UserAgent { 'portscoop-test' }
function strip_filename($u) { $Address }
function get_config($n) { $null }
function filesize($n) { "$n bytes" }
function url_remote_filename($u) { [IO.Path]::GetFileName($u) }
function get_magic_bytes_pretty($f,$s) { 'fixture' }
function Assert($b,$m) { if (-not $b) { throw $m } }
function Reject([scriptblock]$b) { $failed=$false; try { & $b | Out-Null } catch { $failed=$true }; Assert $failed 'Expected rejection' }
$env:PORTSCOOP_ROOT=$Root
$cachedir=Join-Path $Root 'scoop\cache'
$dir=Join-Path $Root 'scoop\apps\fixture\1'
New-Item -ItemType Directory -Path $dir -Force | Out-Null
$url=$Address+'/download?name=wrong.exe'
$manifest=[pscustomobject]@{version='1';url=$url+'#/retained.exe'}
$download=[pscustomobject]@{url=$url;name='retained.exe';hash=$Digest;format='file'}
$portscoopRequest=[pscustomobject]@{manifests=[pscustomobject]@{fixture=[pscustomobject]@{manifest=$manifest;downloads=@($download)}}}
$names=@(Invoke-ScoopDownload fixture 1 $manifest main 64bit $dir)
Assert ($names.Count -eq 1 -and $names[0] -ceq 'retained.exe') 'Frozen name not returned'
Assert ([IO.File]::Exists("$dir\retained.exe")) 'Inert payload not retained'
Assert (-not [IO.File]::Exists("$dir\wrong.exe")) 'Query selected filename'
Reject { Invoke-ScoopDownload fixture 1 $manifest main 64bit $dir $true $false }
$download.hash='0'*64
Reject { Invoke-ScoopDownload fixture 1 $manifest main 64bit $dir }
$download.hash=$Digest
Reject { Invoke-ScoopDownload fixture 1 $manifest main 64bit (Join-Path $Root 'elsewhere') }
$download.name='../escape.exe'
Reject { Invoke-ScoopDownload fixture 1 $manifest main 64bit $dir }
Write-Output 'Frozen download checks passed'
