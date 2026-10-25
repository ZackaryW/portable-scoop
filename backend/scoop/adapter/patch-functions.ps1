# Build-time function replacements. These are written into pinned source files;
# none are monkey-patched into the caller's PowerShell session.
function is_scoop_outdated { return $false }
function Get-DefaultArchitecture { return '64bit' }
function Test-HelperInstalled { param([string]$Helper) return $false }
function Get-InstallationHelper {
    param($Manifest,$Architecture,[switch]$All)
    # All permitted extraction is managed by Portscoop; explicit app dependencies
    # still go through Get-Dependency and the frozen manifest closure.
    return @()
}
function Get-HelperPath {
    param([string]$Helper)
    if ($Helper -ine 'Git') { return $null }
    # The launcher verifies the complete runtime-owned tool inventory.
    $git = Join-Path $env:PORTSCOOP_ROOT '.portscoop\tools\git\cmd\git.exe'
    if (Test-Path -LiteralPath $git -PathType Leaf) { return $git }
    return $null
}
function Invoke-Git {
    param([Alias('PSPath','Path')][string]$WorkingDirectory,[Alias('Args')][string[]]$ArgumentList)
    $git=Get-HelperPath Git
    if (-not $git) { throw 'Managed Store Git is unavailable; repair the Store runtime through its Host' }
    if ($ArgumentList[0] -eq 'clone') { $ArgumentList=@('clone','--no-hardlinks')+@($ArgumentList|Select-Object -Skip 1) }
    if (-not $WorkingDirectory) { $WorkingDirectory=Join-Path $env:PORTSCOOP_ROOT 'scoop\buckets' }
    $ArgumentList=@('-c','credential.helper=','-c','core.fsmonitor=false','-c','protocol.ext.allow=never','-C',$WorkingDirectory)+$ArgumentList
    $saved=@{}
    foreach($key in @('PATH','HOME','XDG_CONFIG_HOME','GIT_CONFIG_GLOBAL','GIT_EXEC_PATH')) { $saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process') }
    try {
        $gitRoot=Join-Path $env:PORTSCOOP_ROOT '.portscoop\tools\git'
        $env:PATH=(Join-Path $gitRoot 'cmd')+';'+(Join-Path $gitRoot 'ucrt64\bin')+';'+(Join-Path $gitRoot 'usr\bin')+';'+(Join-Path $env:SystemRoot 'System32')
        $env:HOME=Join-Path $env:PORTSCOOP_ROOT '.portscoop\state\git-home'
        $env:XDG_CONFIG_HOME=$env:HOME
        $env:GIT_CONFIG_GLOBAL='NUL'
        $env:GIT_EXEC_PATH=Join-Path $gitRoot 'ucrt64\bin'
        & $git @ArgumentList
    } finally {
        foreach($key in $saved.Keys){[Environment]::SetEnvironmentVariable($key,$saved[$key],'Process')}
    }
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
    if ($HookType -cnotin @('pre_install','post_install')) { throw 'No portable replacement for this package hook' }
    $entry = $portscoopRequest.manifests.PSObject.Properties | Where-Object { $_.Name -ieq $app } | Select-Object -First 1
    if (-not $entry -or $entry.Value.manifest.version -cne $Manifest.version -or
        (@($entry.Value.manifest.$HookType) -join "`n") -cne (@($Manifest.$HookType) -join "`n")) {
        throw 'Package hook does not match the frozen manifest'
    }
    $actions = @($entry.Value.actions | Where-Object { $_.phase -ceq $HookType })
    $decision = @($entry.Value.decisions | Where-Object { $_.field -ceq $HookType -and $_.disposition -cin @('rewrite','skip') })
    if ($decision.Count -ne 1 -or (-not $actions.Count -and $decision[0].disposition -cne 'skip')) { throw 'Package hook has no approved operation plan' }
    foreach ($action in $actions) {
        if ($action.persist_path) {
            $mapped = $false
            foreach ($item in @($entry.Value.manifest.persist)) {
                $pair = @($item); $source = [string]$pair[0]; $saved = $source
                if ($pair.Count -gt 1) { $saved = [string]$pair[1] }
                $source=$source.Replace('\','/'); $saved=$saved.Replace('\','/')
                if ($action.path -ieq $source -or $action.path.StartsWith($source+'/',[StringComparison]::OrdinalIgnoreCase)) {
                    if (($saved+$action.path.Substring($source.Length)) -ieq $action.persist_path) { $mapped = $true }
                }
            }
            if (-not $mapped) { throw 'Action persistence mapping differs from frozen manifest' }
        }
        Invoke-PortscoopAction -Action $action -App $app -Version $Manifest.version -Directory $dir -PersistDirectory $persist_dir -PersistMappings @($entry.Value.manifest.persist)
    }
}
function Invoke-ScoopDownload {
    param($app, $version, $manifest, $bucket, $architecture, $dir, $use_cache = $true, $check_hash = $true)
    if (-not $check_hash) { throw 'Download hash verification cannot be disabled' }
    $entry = $portscoopRequest.manifests.PSObject.Properties | Where-Object { $_.Name -ieq $app } | Select-Object -First 1
    if (-not $entry -or $entry.Value.manifest.version -cne $version -or
        (@($entry.Value.manifest.url) -join "`n") -cne (@($manifest.url) -join "`n")) { throw 'Download does not match frozen manifest' }
    $downloads = @($entry.Value.downloads)
    if (-not $downloads.Count) { throw 'Missing frozen download plan' }
    $root = [IO.Path]::GetFullPath($env:PORTSCOOP_ROOT)
    $expected = Join-Path $root "scoop\apps\$app\$version"
    if (-not ([IO.Path]::GetFullPath($dir)).Equals($expected,[StringComparison]::OrdinalIgnoreCase)) { throw 'Download directory differs from Store' }
    foreach ($download in $downloads) {
        $name = [string]$download.name
        if (-not $name -or $name -ne [IO.Path]::GetFileName($name) -or $name.IndexOfAny([IO.Path]::GetInvalidFileNameChars()) -ge 0) { throw 'Invalid frozen download name' }
        $uri = [Uri]$download.url
        if ($uri.Scheme -notin @('http','https') -or $uri.Fragment -or $uri.UserInfo) { throw 'Invalid frozen network URL' }
        $target = Join-Path $expected $name
        Assert-PortscoopActionPath -Root $root -Path $target
        # Explicit filename controls cache/display naming; Invoke-Download strips
        # the fragment before sending the request. Query strings cannot rename it.
        $url = $download.url + '#/' + $name
        Invoke-CachedDownload $app $version $url $target $null $use_cache
        $ok, $errorText = check_hash $target $download.hash $app
        if (-not $ok) { throw "Download digest mismatch: $errorText" }
    }
    return @($downloads | ForEach-Object { [string]$_.name })
}

function Invoke-Extraction {
    param([string]$Path,[string[]]$Name,[psobject]$Manifest,[Alias('Arch','Architecture')][string]$ProcessorArchitecture)
    $entry = $portscoopRequest.manifests.PSObject.Properties | Where-Object { $_.Name -ieq $app } | Select-Object -First 1
    if (-not $entry -or $entry.Value.manifest.version -cne $Manifest.version) { throw 'No frozen extraction manifest' }
    $expected = Join-Path $env:PORTSCOOP_ROOT "scoop\apps\$app\$($Manifest.version)"
    if (-not ([IO.Path]::GetFullPath($Path)).Equals($expected,[StringComparison]::OrdinalIgnoreCase)) { throw 'Extraction directory differs from Store' }
    if (($Name -join "`n") -cne (@($entry.Value.downloads | ForEach-Object { $_.name }) -join "`n")) { throw 'Extraction names differ from frozen plan' }
    # Fixed Store runtime, checked by its runtime receipt and the request identity.
    & (Join-Path $env:PORTSCOOP_ROOT 'portscoop.exe') __portscoop_extract $app
    if ($LASTEXITCODE -ne 0) { throw 'Managed extraction failed' }
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
