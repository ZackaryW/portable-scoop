# Domain binding for the generic tree helpers. Called only after lifecycle,
# physical version/current, and Store identity validation by the action executor.
function Test-PortscoopTreeOverlap {
    param([string]$A,[string]$B)
    return $A.Equals($B,[StringComparison]::OrdinalIgnoreCase) -or $A.StartsWith($B+'/',[StringComparison]::OrdinalIgnoreCase) -or $B.StartsWith($A+'/',[StringComparison]::OrdinalIgnoreCase)
}
function Test-PortscoopCleanupOverlap {
    param([string]$Selection,[string]$Persisted)
    if($Selection.EndsWith('*')) {
        $prefix=$Selection.Substring(0,$Selection.Length-1)
        return $Persisted.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase) -or $prefix.StartsWith($Persisted+'/',[StringComparison]::OrdinalIgnoreCase)
    }
    return Test-PortscoopTreeOverlap $Selection $Persisted
}
function Invoke-PortscoopTreeAction {
    param([psobject]$Action,[string]$Root,[string]$Physical,[string]$Persist,[object[]]$Mappings)
    foreach($flag in @('recursive','missing_ok','force')){if($null -ne $Action.$flag -and $Action.$flag -isnot [bool]){throw 'Invalid tree action flag'}}
    if($Action.guard -and $Action.guard -cne 'source-exists'){throw 'Unsupported tree action guard'}
    $pairs=@()
    foreach($item in $Mappings) {
        $pair=@($item);$from=[string]$pair[0];$to=$from;if($pair.Count -gt 1){$to=[string]$pair[1]}
        Assert-PortscoopRelativePath $from
        Assert-PortscoopRelativePath $to
        $pairs+=,[pscustomobject]@{From=$from.Replace('\','/');To=$to.Replace('\','/')}
    }
    if($Action.kind -ceq 'cleanup-package-tree') {
        $selections=@($Action.paths)
        if(-not $selections.Count -or $selections.Count -gt 256 -or $Action.persist_path -or $Action.path){throw 'Invalid cleanup action selections'}
        foreach($selection in $selections) {
            if($selection -isnot [string] -or $selection.Contains('\')){throw 'Cleanup selection must be a canonical string'}
            foreach($pair in $pairs){if(Test-PortscoopCleanupOverlap $selection $pair.From){throw 'Cleanup overlaps declared persistence'}}
        }
        if($Action.guard) {
            Assert-PortableTreeName $Action.source
            if($selections.Count -ne 1 -or $selections[0] -ine $Action.source){throw 'Cleanup guard differs from selection'}
            $source=Join-Path $Physical $Action.source
            Assert-PortscoopActionPath $Root $source
            if(-not [IO.File]::Exists($source) -and -not [IO.Directory]::Exists($source)){return}
        } elseif($Action.source){throw 'Unexpected cleanup source'}
        Remove-PortableTree $Physical $selections -Recurse:([bool]$Action.recursive) -MissingOK:([bool]$Action.missing_ok) -Force:([bool]$Action.force) -AllowPrefix
        return
    }
    if($Action.kind -cne 'copy-package-tree' -or -not $Action.recursive -or -not $Action.force -or $Action.missing_ok -or $Action.paths){throw 'Invalid recursive copy action'}
    Assert-PortableTreeName $Action.source
    Assert-PortableTreeName $Action.path
    if($Action.source.Contains('\') -or $Action.path.Contains('\')){throw 'Tree action paths must be canonical'}
    $source=Join-Path $Physical $Action.source
    Assert-PortscoopActionPath $Root $source
    $destination=Join-Path $Physical $Action.path
    $mapped=$false
    foreach($pair in $pairs) {
        if(Test-PortscoopTreeOverlap $Action.source $pair.From){throw 'Tree source overlaps persistence'}
        if(Test-PortscoopTreeOverlap $Action.path $pair.From) {
            if($Action.path -ine $pair.From -or $Action.persist_path -ine $pair.To -or $mapped){throw 'Tree destination requires an exact persistence mapping'}
            $mapped=$true
        }
    }
    if($Action.persist_path -and -not $mapped){throw 'Unverified tree persistence mapping'}
    if($mapped -and $Action.phase -ceq 'post_install') {
        $destination=Join-Path $Persist $Action.persist_path
        Assert-PortscoopActionPath $Root $destination
        $linkPath=Join-Path $Physical $Action.path
        Assert-PortscoopActionPath $Root ([IO.Path]::GetDirectoryName($linkPath))
        $link=Get-Item -LiteralPath $linkPath -Force
        $targets=@($link.Target)
        if($link.LinkType -ne 'Junction' -or $targets.Count -ne 1 -or -not ([IO.Path]::GetFullPath($targets[0])).Equals($destination,[StringComparison]::OrdinalIgnoreCase)){throw 'Unverified resource persistence junction'}
    } else {Assert-PortscoopActionPath $Root $destination}
    if(-not [IO.Directory]::Exists($source)) {
        if($Action.guard -and -not [IO.File]::Exists($source)){return}
        throw 'Tree copy source must be a directory'
    }
    Copy-PortableTree $source $destination
}
