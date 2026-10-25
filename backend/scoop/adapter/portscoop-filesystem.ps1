# Checked filesystem mechanics. These functions know ordinary roots and paths,
# not manifests or Store layouts. Callers authorize persistence destinations.
# Every selection is validated before mutation. Copy commits individual files;
# execution failures report completed entries and do not promise tree rollback.
function Assert-PortableTreeName {
    param([string]$Relative)
    if (-not $Relative -or [IO.Path]::IsPathRooted($Relative) -or $Relative -match '[<>:"|?*\[\]\x00-\x1f]') {throw 'Invalid literal tree path'}
    foreach($part in ($Relative -split '[\\/]')) {
        if (-not $part -or $part -in @('.','..') -or $part.EndsWith('.') -or $part.EndsWith(' ') -or $part.Split('.')[0] -match '\A(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])\z') {throw 'Invalid literal tree component'}
    }
}
function Get-PortableTreeRoot {
    param([string]$Root)
    if (-not [IO.Path]::IsPathRooted($Root) -or $Root.StartsWith('\\')) {throw 'Tree root must be a local absolute directory'}
    $full=[IO.Path]::GetFullPath($Root).TrimEnd('\')
    if ($full.Length -le 2) {throw 'Volume roots are not tree operation roots'}
    Assert-PortableTreePath $full $full
    if ([IO.File]::Exists($full)) {throw 'Tree root is a file'}
    return $full
}
function Assert-PortableTreePath {
    param([string]$Root,[string]$Path)
    $full=[IO.Path]::GetFullPath($Path)
    if (-not $full.Equals($Root,[StringComparison]::OrdinalIgnoreCase) -and -not $full.StartsWith($Root+'\',[StringComparison]::OrdinalIgnoreCase)) {throw 'Tree path escapes root'}
    $cursor=$full
    while($cursor) {
        try {$attributes=[IO.File]::GetAttributes($cursor)}
        catch [IO.FileNotFoundException] {$attributes=0}
        catch [IO.DirectoryNotFoundException] {$attributes=0}
        if (($attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {throw 'Tree path contains a reparse point'}
        $cursor=[IO.Path]::GetDirectoryName($cursor)
    }
}
function Get-PortableTreeChildren {
    param([string]$Path,[int]$Limit)
    $names=New-Object 'Collections.Generic.List[string]'
    foreach($name in [IO.Directory]::EnumerateFileSystemEntries($Path)) {
        if ($names.Count -ge $Limit) {throw 'Tree entry count exceeds limit'}
        $names.Add($name)
    }
    $array=$names.ToArray()
    [Array]::Sort($array,[StringComparer]::Ordinal)
    return ,$array
}
function Get-PortableTreePlan {
    # Selections are literal relative paths, or terminal nonempty prefixes when
    # explicitly enabled. No evaluation or wildcard traversal is performed.
    param([string]$Root,[string[]]$Selections,[switch]$Recurse,[switch]$MissingOK,[switch]$AllowPrefix,
        [ValidateRange(1,100000)][int]$MaxEntries=100000,[ValidateRange(1,64)][int]$MaxDepth=64,
        [ValidateRange(1,8589934592)][long]$MaxBytes=8589934592,[ValidateRange(1,4294967296)][long]$MaxFileBytes=4294967296)
    $Root=Get-PortableTreeRoot $Root
    $entries=New-Object 'Collections.Generic.Dictionary[string,object]' ([StringComparer]::OrdinalIgnoreCase)
    $pending=New-Object 'Collections.Generic.Stack[string]'
    foreach($selection in $Selections) {
        $selection=$selection.Replace('/','\')
        if ($selection.EndsWith('*') -and $AllowPrefix) {
            $literal=$selection.Substring(0,$selection.Length-1)
            Assert-PortableTreeName $literal
            $prefix=[IO.Path]::GetFileName($literal)
            if (-not $prefix) {throw 'Root-wide wildcard is unsupported'}
            $parent=[IO.Path]::GetDirectoryName($literal)
            $directory=$Root;if($parent){$directory=Join-Path $Root $parent}
            Assert-PortableTreePath $Root $directory
            $matches=0
            if ([IO.Directory]::Exists($directory)) {
                foreach($path in (Get-PortableTreeChildren $directory $MaxEntries)) {
                    if ([IO.Path]::GetFileName($path).StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)) {$pending.Push($path);$matches++}
                }
            } elseif ([IO.File]::Exists($directory)) {throw 'Wildcard parent is a file'}
            if (-not $matches -and -not $MissingOK) {throw 'Tree selection has no matches'}
        } else {
            Assert-PortableTreeName $selection
            $path=Join-Path $Root $selection
            Assert-PortableTreePath $Root $path
            try {$null=[IO.File]::GetAttributes($path);$pending.Push($path)}
            catch [IO.FileNotFoundException] {if(-not $MissingOK){throw}}
            catch [IO.DirectoryNotFoundException] {if(-not $MissingOK){throw}}
        }
        if ($pending.Count -gt $MaxEntries) {throw 'Tree selection count exceeds limit'}
    }
    [long]$bytes=0
    while($pending.Count) {
        $path=$pending.Pop()
        Assert-PortableTreePath $Root $path
        $relative=$path.Substring($Root.Length+1)
        Assert-PortableTreeName $relative
        if ($entries.ContainsKey($relative)) {
            if ($entries[$relative].Relative -cne $relative) {throw 'Case-colliding tree entries'}
            continue
        }
        if ($entries.Count -ge $MaxEntries) {throw 'Tree entry count exceeds limit'}
        $depth=($relative -split '\\').Count
        if ($depth -gt $MaxDepth) {throw 'Tree depth exceeds limit'}
        $attributes=[IO.File]::GetAttributes($path)
        $directory=($attributes -band [IO.FileAttributes]::Directory) -ne 0
        [long]$length=0
        if ($directory) {
            $children=Get-PortableTreeChildren $path $MaxEntries
            if ($children.Count -and -not $Recurse) {throw 'Nonempty tree selection requires recursion'}
            foreach($child in $children){$pending.Push($child)}
            if($pending.Count -gt $MaxEntries){throw 'Tree entry count exceeds limit'}
        } else {
            $length=([IO.FileInfo]$path).Length
            if ($length -gt $MaxFileBytes -or $bytes -gt $MaxBytes-$length) {throw 'Tree byte limit exceeded'}
            $bytes+=$length
        }
        $entries.Add($relative,[pscustomobject]@{Relative=$relative;Directory=$directory;Length=$length;Depth=$depth})
    }
    [string[]]$ordered=@($entries.Keys)
    [Array]::Sort($ordered,[StringComparer]::Ordinal)
    $result=New-Object 'Collections.Generic.List[object]'
    foreach($key in $ordered){$result.Add($entries[$key])}
    return [pscustomobject]@{Root=$Root;Entries=$result.ToArray();Bytes=$bytes}
}
function Copy-PortableTree {
    # Source contents replace colliding regular files atomically. Unrelated
    # destination entries remain. Reparse points and type conflicts fail before
    # the first write; later I/O errors may leave completed file replacements.
    param([string]$SourceRoot,[string]$DestinationRoot,
        [ValidateRange(1,100000)][int]$MaxEntries=100000,[ValidateRange(1,64)][int]$MaxDepth=64,
        [ValidateRange(1,8589934592)][long]$MaxBytes=8589934592,[ValidateRange(1,4294967296)][long]$MaxFileBytes=4294967296)
    $SourceRoot=Get-PortableTreeRoot $SourceRoot
    $DestinationRoot=Get-PortableTreeRoot $DestinationRoot
    if ($SourceRoot.Equals($DestinationRoot,[StringComparison]::OrdinalIgnoreCase) -or $DestinationRoot.StartsWith($SourceRoot+'\',[StringComparison]::OrdinalIgnoreCase) -or $SourceRoot.StartsWith($DestinationRoot+'\',[StringComparison]::OrdinalIgnoreCase)) {throw 'Copy roots overlap'}
    if (-not [IO.Directory]::Exists($SourceRoot)) {throw 'Copy source directory is missing'}
    $selections=@(Get-PortableTreeChildren $SourceRoot $MaxEntries | ForEach-Object {$_} | ForEach-Object {[IO.Path]::GetFileName($_)})
    $plan=Get-PortableTreePlan $SourceRoot $selections -Recurse -MaxEntries $MaxEntries -MaxDepth $MaxDepth -MaxBytes $MaxBytes -MaxFileBytes $MaxFileBytes
    foreach($entry in $plan.Entries) {
        $target=Join-Path $DestinationRoot $entry.Relative
        Assert-PortableTreePath $DestinationRoot $target
        if (($entry.Directory -and [IO.File]::Exists($target)) -or (-not $entry.Directory -and [IO.Directory]::Exists($target))) {throw 'Copy destination type conflict'}
        $parent=[IO.Path]::GetDirectoryName($target)
        while($parent.Length -ge $DestinationRoot.Length){if([IO.File]::Exists($parent)){throw 'Copy ancestor is a file'};if($parent -ceq $DestinationRoot){break};$parent=[IO.Path]::GetDirectoryName($parent)}
    }
    $completed=0
    try {
        [IO.Directory]::CreateDirectory($DestinationRoot) | Out-Null
        foreach($entry in $plan.Entries) {
            $source=Join-Path $SourceRoot $entry.Relative
            $target=Join-Path $DestinationRoot $entry.Relative
            Assert-PortableTreePath $SourceRoot $source
            Assert-PortableTreePath $DestinationRoot $target
            if($entry.Directory){[IO.Directory]::CreateDirectory($target)|Out-Null;continue}
            [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($target)) | Out-Null
            $temporary=Join-Path ([IO.Path]::GetDirectoryName($target)) ('.portscoop-copy-'+[guid]::NewGuid().ToString('N'))
            try {
                $input=[IO.File]::Open($source,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
                try {
                    if($input.Length -ne $entry.Length){throw 'Copy source size changed'}
                    $output=[IO.File]::Open($temporary,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
                    try {$input.CopyTo($output);$output.Flush($true)} finally {$output.Dispose()}
                } finally {$input.Dispose()}
                if([IO.File]::Exists($target)){[IO.File]::Replace($temporary,$target,[NullString]::Value)}else{[IO.File]::Move($temporary,$target)}
                $completed++
            } finally {if([IO.File]::Exists($temporary)){[IO.File]::Delete($temporary)}}
        }
    } catch {throw "Checked copy stopped after $completed files: $($_.Exception.Message)"}
}
function Remove-PortableTree {
    param([string]$Root,[string[]]$Selections,[switch]$Recurse,[switch]$MissingOK,[switch]$AllowPrefix,[switch]$Force,
        [ValidateRange(1,100000)][int]$MaxEntries=100000,[ValidateRange(1,64)][int]$MaxDepth=64,
        [ValidateRange(1,8589934592)][long]$MaxBytes=8589934592,[ValidateRange(1,4294967296)][long]$MaxFileBytes=4294967296)
    $plan=Get-PortableTreePlan $Root $Selections -Recurse:$Recurse -MissingOK:$MissingOK -AllowPrefix:$AllowPrefix -MaxEntries $MaxEntries -MaxDepth $MaxDepth -MaxBytes $MaxBytes -MaxFileBytes $MaxFileBytes
    $completed=0
    try {
        [object[]]$ordered=@($plan.Entries)
        $comparison=[Comparison[object]]{param($a,$b) if($a.Depth -ne $b.Depth){return $b.Depth.CompareTo($a.Depth)};return [StringComparer]::Ordinal.Compare($a.Relative,$b.Relative)}
        [Array]::Sort($ordered,[Collections.Generic.Comparer[object]]::Create($comparison))
        foreach($entry in $ordered) {
            $path=Join-Path $plan.Root $entry.Relative
            Assert-PortableTreePath $plan.Root $path
            $attributes=[IO.File]::GetAttributes($path)
            if ((($attributes -band [IO.FileAttributes]::Directory) -ne 0) -ne $entry.Directory) {throw 'Cleanup entry type changed'}
            if ($Force -and ($attributes -band [IO.FileAttributes]::ReadOnly)) {[IO.File]::SetAttributes($path,($attributes -band (-bnot [IO.FileAttributes]::ReadOnly)))}
            if($entry.Directory){[IO.Directory]::Delete($path,$false)}else{[IO.File]::Delete($path)}
            $completed++
        }
    } catch {throw "Checked cleanup stopped after $completed entries: $($_.Exception.Message)"}
}
