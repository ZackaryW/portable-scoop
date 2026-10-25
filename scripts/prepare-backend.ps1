[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$pin = Get-Content -LiteralPath (Join-Path $projectRoot 'backend\scoop\upstream.json') -Raw | ConvertFrom-Json
$source = & (Join-Path $PSScriptRoot 'fetch-scoop.ps1')
if (Test-Path -LiteralPath $Destination) { throw 'Backend destination must not exist' }
$Destination = [IO.Path]::GetFullPath($Destination)
$engine = Join-Path $Destination 'engine'
$adapter = Join-Path $Destination 'adapter'
New-Item -ItemType Directory -Path $engine,$adapter -Force | Out-Null
Get-ChildItem -LiteralPath $source -Force | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination $engine -Recurse }

$patchFile = Join-Path $projectRoot 'backend\scoop\adapter\patch-functions.ps1'
$tokens = $null; $parseErrors = $null
$patchAst = [Management.Automation.Language.Parser]::ParseFile($patchFile, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw 'Invalid patch source' }
$replacements = @{}
$patchAst.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false) | ForEach-Object { $replacements[$_.Name] = $_.Extent.Text }
$sites = @{
    'lib/core.ps1' = @('is_scoop_outdated','Get-DefaultArchitecture','Test-HelperInstalled','Get-HelperPath','Invoke-Git')
    'lib/install.ps1' = @('create_shims','Invoke-HookScript','Invoke-Installer')
    'lib/shortcuts.ps1' = @('create_startmenu_shortcuts','rm_startmenu_shortcuts','shortcut_folder','startmenu_shortcut')
    'lib/psmodules.ps1' = @('install_psmodule','uninstall_psmodule','ensure_in_psmodulepath')
    'lib/commands.ps1' = @('command_path')
    'lib/buckets.ps1' = @('Convert-RepositoryUri')
    'lib/manifest.ps1' = @('Get-Manifest','manifest','save_installed_manifest')
    'libexec/scoop-update.ps1' = @('Sync-Scoop','Sync-Bucket')
}
foreach ($relative in $sites.Keys) {
    $file = Join-Path $engine $relative
    if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $pin.source_files.$relative) { throw "Pinned patch input changed: $relative" }
    $text = [IO.File]::ReadAllText($file)
    $ast = [Management.Automation.Language.Parser]::ParseFile($file, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count) { throw "Invalid upstream source: $relative" }
    $functions = @($ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false) | Where-Object { $_.Name -in $sites[$relative] })
    if ($functions.Count -ne $sites[$relative].Count) { throw "Patch sites changed: $relative" }
    foreach ($function in ($functions | Sort-Object { $_.Extent.StartOffset } -Descending)) {
        $text = $text.Substring(0,$function.Extent.StartOffset) + $replacements[$function.Name] + $text.Substring($function.Extent.EndOffset)
    }
    [IO.File]::WriteAllText($file,$text,(New-Object Text.UTF8Encoding($false)))
}
Copy-Item -LiteralPath (Join-Path $projectRoot 'backend\scoop\adapter\system.ps1') -Destination (Join-Path $engine 'lib\system.ps1') -Force
$bucketFile=Join-Path $engine 'lib\buckets.ps1'
$bucketText=[IO.File]::ReadAllText($bucketFile)
$oldDiagnostic="Git is required for buckets. Run 'scoop install git' and try again."
if (-not $bucketText.Contains($oldDiagnostic)){throw 'Git prerequisite diagnostic patch site changed'}
$bucketText=$bucketText.Replace($oldDiagnostic,'Standalone Git is required at %ProgramFiles%\Git\cmd\git.exe for bucket operations.')
[IO.File]::WriteAllText($bucketFile,$bucketText,(New-Object Text.UTF8Encoding($false)))
Copy-Item -LiteralPath (Join-Path $projectRoot 'backend\scoop\adapter\entry.ps1') -Destination $adapter
Copy-Item -LiteralPath (Join-Path $projectRoot 'internal\runner\request-reader.ps1') -Destination $adapter
$artifacts = [ordered]@{}
Get-ChildItem -LiteralPath $engine -File -Recurse | Sort-Object FullName | ForEach-Object {
    $relative = $_.FullName.Substring($engine.Length+1).Replace('\','/')
    $artifacts["scoop/apps/scoop/current/$relative"] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
}
Get-ChildItem -LiteralPath $adapter -File | Sort-Object FullName | ForEach-Object {
    $artifacts[".portscoop/backend/$($_.Name)"] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
}
$metadata = @{ schema_version=1; protocol_major=1; upstream_revision=$pin.upstream_revision; patch_revision='portable-zip-v1.1'; artifacts=$artifacts } | ConvertTo-Json -Depth 5
[IO.File]::WriteAllText((Join-Path $Destination 'backend.json'),$metadata,(New-Object Text.UTF8Encoding($false)))
Write-Output $Destination
