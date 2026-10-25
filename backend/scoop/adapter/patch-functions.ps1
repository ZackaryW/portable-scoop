# Build-time function replacements. These are written into pinned source files;
# none are monkey-patched into the caller's PowerShell session.
function is_scoop_outdated { return $false }
function Get-DefaultArchitecture { return '64bit' }
function Test-HelperInstalled { param([string]$Helper) return $false }
function Get-HelperPath {
    param([string]$Helper)
    if ($Helper -ine 'Git') { return $null }
    # Git is an explicit standalone prerequisite for Git-backed buckets.
    $git = Join-Path $env:ProgramFiles 'Git\cmd\git.exe'
    if (Test-Path -LiteralPath $git -PathType Leaf) { return $git }
    return $null
}
function Invoke-Git {
    param([Alias('PSPath','Path')][string]$WorkingDirectory,[Alias('Args')][string[]]$ArgumentList)
    $git=Get-HelperPath Git
    if (-not $git) { throw 'Standalone Git is required at %ProgramFiles%\Git\cmd\git.exe' }
    if ($ArgumentList[0] -eq 'clone') { $ArgumentList=@('clone','--no-hardlinks')+@($ArgumentList|Select-Object -Skip 1) }
    if (-not $WorkingDirectory) { $WorkingDirectory=Join-Path $env:PORTSCOOP_ROOT 'scoop\buckets' }
    $ArgumentList=@('-c','credential.helper=','-C',$WorkingDirectory)+$ArgumentList
    & $git @ArgumentList
}
function Convert-RepositoryUri {
    param([string]$Uri)
    if ([IO.Path]::IsPathRooted($Uri)) { return 'file/'+[IO.Path]::GetFullPath($Uri).TrimEnd('\','/').ToLowerInvariant() }
    $parsed=$null
    if ([Uri]::TryCreate($Uri,[UriKind]::Absolute,[ref]$parsed) -and $parsed.Scheme -in @('http','https')) { return $parsed.AbsoluteUri.TrimEnd('/') -replace '\.git$','' }
    throw 'Only HTTP(S) or absolute local Git repositories are supported'
}
function create_shims($manifest, $dir, $global, $arch) {
    if ($global) { throw 'Global shims are disabled' }
    foreach ($item in @(arch_specific 'bin' $manifest $arch)) {
        if ($null -eq $item) { continue }
        $target, $name, $arg = shim_def $item
        $bin = Join-Path $dir $target
        if (-not (Test-Path -LiteralPath $bin -PathType Leaf)) { throw "Portable shim target is missing: $target" }
        if (-not (is_in_dir $dir (Convert-Path -LiteralPath $bin))) { throw 'Shim target escaped its package' }
        shim $bin $false $name (substitute $arg @{ '$dir' = $dir; '$original_dir' = $original_dir; '$persist_dir' = $persist_dir })
    }
}
function create_startmenu_shortcuts($manifest, $dir, $global, $arch) {
    if ($global -or (arch_specific 'shortcuts' $manifest $arch)) { throw 'Shortcut declarations must be compiled to portable aliases' }
}
function rm_startmenu_shortcuts($manifest, $global, $arch) { }
function shortcut_folder($global) { throw 'Host shortcuts are disabled in Portscoop' }
function startmenu_shortcut { throw 'Host shortcuts are disabled in Portscoop' }
function install_psmodule($manifest, $dir, $global) {
    if ($global -or $manifest.psmodule) { throw 'Portable module discovery is not implemented' }
}
function uninstall_psmodule($manifest, $dir, $global) {
    if ($global -or $manifest.psmodule) { throw 'Portable module discovery is not implemented' }
}
function ensure_in_psmodulepath($dir, $global) { throw 'Portable module discovery is not implemented' }
function Sync-Scoop { throw 'Engine updates require a compatible Portscoop release' }
function Sync-Bucket { throw 'Implicit bucket updates are disabled during app updates' }
function Invoke-HookScript {
	param([string]$HookType, [psobject]$Manifest, [Alias('Arch','Architecture')][string]$ProcessorArchitecture)
    if (-not (arch_specific $HookType $Manifest $ProcessorArchitecture)) { return }
    if ($HookType -cne 'pre_install') { throw 'No portable replacement for this package hook' }
    $entry = $portscoopRequest.manifests.PSObject.Properties | Where-Object { $_.Name -ieq $app } | Select-Object -First 1
    if (-not $entry -or $entry.Value.manifest.version -cne $Manifest.version -or
        (@($entry.Value.manifest.pre_install) -join "`n") -cne (@($Manifest.pre_install) -join "`n")) {
        throw 'Package hook does not match the frozen manifest'
    }
    $actions = @($entry.Value.actions | Where-Object { $_.phase -ceq $HookType })
    if ($actions.Count -ne 1) { throw 'Package hook has no unique approved action' }
    Invoke-PortscoopAction -Action $actions[0] -App $app -Version $Manifest.version -Directory $dir -PersistDirectory $persist_dir
}
function Invoke-Installer {
    param([string]$Path, [string[]]$Name, [psobject]$Manifest, [Alias('Arch','Architecture')][string]$ProcessorArchitecture, [string]$AppName, [switch]$Global, [switch]$Uninstall)
    if ($Global -or (arch_specific 'installer' $Manifest $ProcessorArchitecture) -or (arch_specific 'uninstaller' $Manifest $ProcessorArchitecture)) {
        throw 'Package installers are disabled in Portscoop'
    }
}
function command_path($cmd) {
    if ($cmd -notin @('install','uninstall','update','list','search','status','bucket','reset','cleanup','config')) { throw 'Unsupported portable command' }
    return "$PSScriptRoot\..\libexec\scoop-$cmd.ps1"
}
function Get-Manifest($app) {
    $name, $bucket, $version = parse_app $app
    if ($version) { throw 'Generated version manifests are disabled' }
    $entry = $portscoopRequest.manifests.PSObject.Properties | Where-Object { $_.Name -ieq $name } | Select-Object -First 1
    if (-not $entry) { throw "Package was not preflighted: $app" }
    return $name, $entry.Value.manifest, $entry.Value.bucket, $null
}
function manifest($app, $bucket, $url) {
    $null, $data, $null, $null = Get-Manifest $app
    return $data
}
function save_installed_manifest($app, $bucket, $dir, $url) {
    $null, $data, $null, $null = Get-Manifest $app
    $data | ConvertTo-Json -Depth 40 | Out-UTF8File (Join-Path $dir 'scoop-manifest.json')
}
