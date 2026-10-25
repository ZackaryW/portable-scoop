#Requires -Version 5.1
[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
$machine=Get-CimInstance Win32_ComputerSystem
if ($machine.Manufacturer -notmatch 'VMware' -and $machine.Model -notmatch 'VMware'){throw 'This suite runs only in the selected disposable VMware guest'}
$root=$PSScriptRoot
$results=Join-Path $root 'results'
New-Item -ItemType Directory -Path $results -Force | Out-Null
$observations=[ordered]@{schema_version=1;status='running';windows=[Environment]::OSVersion.Version.ToString();powershell=$PSVersionTable.PSVersion.ToString();go_absent=(-not (Get-Command go -ErrorAction SilentlyContinue));error=$null}
$server=$null
$originalUserPath=[Environment]::GetEnvironmentVariable('PATH','User')
$originalProcessPath=$env:PATH
$originalScoop=$env:SCOOP
$originalGlobal=$env:SCOOP_GLOBAL
$originalCache=$env:SCOOP_CACHE
$driveLetters=@()
$vhd=$null
function Write-JSON([string]$Path,$Value){[IO.File]::WriteAllText($Path,($Value|ConvertTo-Json -Depth 40),(New-Object Text.UTF8Encoding($false)))}
function Invoke-PSP([string]$Executable,[string[]]$Arguments,[string]$Log,[switch]$FailureExpected){
    $previousPreference=$ErrorActionPreference
    $ErrorActionPreference='Continue'
    try {$output=& $Executable @Arguments 2>&1; $code=$LASTEXITCODE} finally {$ErrorActionPreference=$previousPreference}
    $output | Out-String | Set-Content (Join-Path $results ($Log+'.log')) -Encoding UTF8
    if ($FailureExpected){if($code-eq 0){throw "$Log unexpectedly succeeded"}}elseif($code-ne 0){throw "$Log failed ($code): $output"}
    return $output
}
function Tree-Digest([string]$Path){
    $records=@(Get-ChildItem -LiteralPath $Path -File -Recurse | ForEach-Object {($_.FullName.Substring($Path.Length))+':'+(Get-FileHash -LiteralPath $_.FullName).Hash})
    return ($records -join "`n")
}
function Invoke-Diskpart([string[]]$Lines,[string]$Name){
    $file=Join-Path $root ($Name+'.txt')
    [IO.File]::WriteAllLines($file,$Lines,[Text.Encoding]::ASCII)
    & "$env:SystemRoot\System32\diskpart.exe" /s $file | Out-String | Set-Content "$results\$Name.log" -Encoding UTF8
    if($LASTEXITCODE-ne 0){throw "Diskpart fixture failed: $Name"}
}
try {
    $gitRuntime=Join-Path $root 'git-runtime.zip'
    $git=Join-Path $env:ProgramFiles 'Git\cmd\git.exe'
    if ((Test-Path $gitRuntime)-and-not(Test-Path $git)) {Expand-Archive -LiteralPath $gitRuntime -DestinationPath (Join-Path $env:ProgramFiles 'Git')}
    if(-not(Test-Path $git)){throw 'Standalone Git prerequisite must be staged for bucket qualification'}
    $fixtureGitConfig=Join-Path $root 'fixture-git.config';[IO.File]::WriteAllText($fixtureGitConfig,'')
    $env:GIT_CONFIG_GLOBAL=$fixtureGitConfig;$env:GIT_CONFIG_NOSYSTEM='1'
    $observations.git_version=(& $git --version|Out-String).Trim()
    . (Join-Path $root 'host-state.ps1') | ConvertTo-Json -Depth 6 | Set-Content "$results\host-before.json" -Encoding UTF8
    $release=Join-Path $root 'release'
    Expand-Archive -LiteralPath (Join-Path $root 'release.zip') -DestinationPath $release
    $central=Join-Path $root 'central launcher'
    & (Join-Path $release 'setup.ps1') -Mode Central -Destination $central -AddToPath *> "$results\central-setup.log"
    $expectedPath=$originalUserPath
    if($expectedPath-and-not$expectedPath.EndsWith(';')){$expectedPath+=';'}
    $expectedPath+=$central
    $observations.central_setup_only_requested_path=[Environment]::GetEnvironmentVariable('PATH','User') -ceq $expectedPath
    [Environment]::SetEnvironmentVariable('PATH',$originalUserPath,'User')
    $work=Join-Path $root 'work (portable)'
    $personal=Join-Path $root 'personal (portable)'
    $env:LOCALAPPDATA=Join-Path $root 'host-profile'
    $centralExe=Join-Path $central 'portscoop.exe'
    $null=Invoke-PSP $centralExe @('env','init',$work,'--name','work') 'init-work'
    & (Join-Path $release 'setup.ps1') -Mode Portable -Destination $personal -Name personal *> "$results\portable-setup.log"
    $observations.portable_setup_no_user_path=[Environment]::GetEnvironmentVariable('PATH','User') -ceq $originalUserPath
    $observations.portable_setup_no_host_registry=-not(Test-Path (Join-Path $env:LOCALAPPDATA 'portscoop\registry.json'))
    $workID=(Get-Content "$work\.portscoop\environment.json" -Raw|ConvertFrom-Json).id
    $personalID=(Get-Content "$personal\.portscoop\environment.json" -Raw|ConvertFrom-Json).id
    $observations.unique_identities=$workID -ne $personalID
    $null=Invoke-PSP $centralExe @('env','register',$work,'--name','work') 'register-work'
    $null=Invoke-PSP $centralExe @('list') 'missing-default' -FailureExpected
    $null=Invoke-PSP $centralExe @('env','default','work') 'default-work'
    $ordinary=Join-Path $root 'ordinary scoop'
    New-Item -ItemType Directory -Path "$ordinary\shims","$ordinary\apps\scoop\current\bin","$ordinary\apps\dep\current" -Force | Out-Null
    'ordinary sentinel' | Set-Content "$ordinary\apps\dep\current\tool.cmd"
    'ordinary engine sentinel' | Set-Content "$ordinary\apps\scoop\current\bin\scoop.ps1"
    '@echo off',('echo invoked > "'+$root+'\ordinary-invoked.txt"') | Set-Content "$ordinary\shims\scoop.cmd" -Encoding ASCII
    $ordinaryBefore=Tree-Digest $ordinary
    $env:SCOOP=$ordinary;$env:SCOOP_GLOBAL="$ordinary\global";$env:SCOOP_CACHE="$ordinary\cache";$env:PATH="$ordinary\shims;"+$env:PATH
    $payload=Join-Path $root 'payload';$contents=Join-Path $root 'contents'
    New-Item -ItemType Directory -Path $payload,"$contents\tools","$contents\data" -Force | Out-Null
    '@echo off','echo fixture' | Set-Content "$contents\tools\probe.cmd" -Encoding ASCII
    'independent persisted data' | Set-Content "$contents\data\seed.txt"
    Compress-Archive -Path "$contents\*" -DestinationPath "$payload\package.zip"
    $ready=Join-Path $root 'server-ready.json'
    $server=Start-Process -FilePath (Join-Path $root 'probe-server.exe') -ArgumentList @('-root',('"{0}"'-f $payload),'-ready-file',('"{0}"'-f $ready)) -PassThru -WindowStyle Hidden
    $deadline=[DateTime]::UtcNow.AddSeconds(10)
    while(-not(Test-Path $ready)){if($server.HasExited-or[DateTime]::UtcNow-gt$deadline){throw 'Fixture HTTP server did not start'};Start-Sleep -Milliseconds 100}
    $url=(Get-Content $ready -Raw|ConvertFrom-Json).url
    $hash=(Get-FileHash "$payload\package.zip").Hash.ToLowerInvariant()
    $manifest=@{version='1.0';url="$url/package.zip";hash=$hash;bin=@(,@('tools\probe.cmd','fixture-tool'));persist=@('data');env_set=@{PSP_FIXTURE_TOOL='$dir'};env_add_path=@('tools');depends='dep';shortcuts=@(,@('tools\probe.cmd','PSP CLI fixture'));psmodule=@{name='PspCliFixture'}}
    $dependency=@{version='1.0';url="$url/package.zip";hash=$hash;bin=@(,@('tools\probe.cmd','fixture-dependency'))}
    foreach($portable in @($work,$personal)){
        New-Item -ItemType Directory -Path "$portable\scoop\buckets\fixture\bucket" -Force | Out-Null
        Write-JSON "$portable\scoop\buckets\fixture\bucket\tool.json" $manifest
        Write-JSON "$portable\scoop\buckets\fixture\bucket\dep.json" $dependency
    }
    $null=Invoke-PSP $centralExe @('install','fixture/tool') 'delegated-install-work'
    $observations.dependency_is_root_local=Test-Path "$work\scoop\apps\dep\1.0\tools\probe.cmd"
    $manifest.version='1.5';Write-JSON "$personal\scoop\buckets\fixture\bucket\tool.json" $manifest
    $personalExe=Join-Path $personal 'psp.exe'
    $null=Invoke-PSP $personalExe @('install','tool') 'direct-install-personal'
    $hostRegistry=Get-Content (Join-Path $env:LOCALAPPDATA 'portscoop\registry.json') -Raw|ConvertFrom-Json
    $observations.direct_without_registration=-not$hostRegistry.entries.personal
    $null=Invoke-PSP $centralExe @('env','register',$personal,'--name','personal') 'register-personal'
    'personal data' | Set-Content "$personal\scoop\persist\tool\data\seed.txt"
    $personalBefore=Tree-Digest $personal
    $manifest.version='2.0';Write-JSON "$work\scoop\buckets\fixture\bucket\tool.json" $manifest
    $null=Invoke-PSP $centralExe @('update','tool') 'delegated-update-work'
    $observations.work_updated=Test-Path "$work\scoop\apps\tool\2.0\tools\probe.cmd"
    $null=Invoke-PSP $centralExe @('status') 'status-work'
    $null=Invoke-PSP $centralExe @('search','tool') 'search-work'
    $null=Invoke-PSP $centralExe @('bucket','list') 'buckets-work'
    $null=Invoke-PSP $centralExe @('config') 'config-work'
    $null=Invoke-PSP $centralExe @('reset','tool') 'reset-work'
    $null=Invoke-PSP $centralExe @('cleanup','tool') 'cleanup-work'
    $oldWork=$work;$work=Join-Path $root 'moved work (portable)'
    Move-Item -LiteralPath $oldWork -Destination $work
    $null=Invoke-PSP (Join-Path $work 'psp.exe') @('list') 'direct-after-directory-move'
    $shim=Get-Content "$work\scoop\shims\fixture-tool.cmd" -Raw
    $observations.directory_move_repaired=$shim.Contains($work)-and-not$shim.Contains($oldWork)
    $null=Invoke-PSP $centralExe @('env','register',$work,'--name','work') 'reregister-directory-move'
    $null=Invoke-PSP $centralExe @('uninstall','tool') 'delegated-uninstall-work'
    $observations.work_removed=-not(Test-Path "$work\scoop\apps\tool")
    $observations.work_persistence_preserved=Test-Path "$work\scoop\persist\tool\data\seed.txt"
    $observations.personal_unchanged=(Tree-Digest $personal)-ceq$personalBefore
    $observations.ordinary_scoop_unchanged=(Tree-Digest $ordinary)-ceq$ordinaryBefore
    $observations.no_system_scoop_fallback=-not(Test-Path "$root\ordinary-invoked.txt")
    foreach($args in @(@('install','tool','--global'),@('update'),@('install','tool','--skip-hash-check'),@('config','root_path','X:\elsewhere'))){$null=Invoke-PSP $centralExe $args ('reject-'+[guid]::NewGuid().ToString('N')) -FailureExpected}
    foreach($name in @('portscoop.exe','psp.exe')){
        $output=Invoke-PSP (Join-Path $central $name) @('--version') ('powershell-'+$name)
        $batch=Join-Path $root ('smoke-'+$name+'.cmd')
        [IO.File]::WriteAllText($batch,('@"'+(Join-Path $central $name)+'" --version'),[Text.Encoding]::ASCII)
        $cmdOutput=& "$env:SystemRoot\System32\cmd.exe" /d /c $batch
        if($LASTEXITCODE-ne 0){throw 'CMD launcher version smoke failed'}
        $observations[('cmd_'+$name.Replace('.','_'))]=($cmdOutput|Out-String).Trim()-eq($output|Out-String).Trim()
    }
    $duplicate=Join-Path $root 'duplicate identity'
    New-Item -ItemType Directory -Path "$duplicate\.portscoop" -Force | Out-Null
    Copy-Item -LiteralPath "$personal\.portscoop\environment.json" -Destination "$duplicate\.portscoop\environment.json"
    Copy-Item -LiteralPath "$personal\portscoop.exe" -Destination $duplicate
    $null=Invoke-PSP $centralExe @('env','register',$duplicate,'--name','duplicate') 'register-duplicate'
    $null=Invoke-PSP $centralExe @('--env','personal','list') 'duplicate-rejected' -FailureExpected
    $null=Invoke-PSP $centralExe @('env','unregister','duplicate') 'remove-duplicate-alias'
    $null=Invoke-PSP $personalExe @('--env','work','list') 'portable-cross-identity-rejected' -FailureExpected
    $observations.duplicate_and_cross_identity_rejected=$true
    $bad=Join-Path $root 'bad metadata'
    New-Item -ItemType Directory -Path "$bad\.portscoop" -Force | Out-Null
    Copy-Item -LiteralPath "$personal\portscoop.exe" -Destination $bad
    'invalid json' | Set-Content "$bad\.portscoop\environment.json"
    $null=Invoke-PSP "$bad\portscoop.exe" @('list') 'corrupt-marker-rejected' -FailureExpected
    $badIdentity=Get-Content "$personal\.portscoop\environment.json" -Raw|ConvertFrom-Json
    $badIdentity.protocol_major=2;Write-JSON "$bad\.portscoop\environment.json" $badIdentity
    $null=Invoke-PSP "$bad\portscoop.exe" @('list') 'protocol-rejected' -FailureExpected
    $observations.corrupt_and_protocol_rejected=$true
    $unsafe=@{version='1';pre_install='throw "must never run"'}
    Write-JSON "$work\scoop\buckets\fixture\bucket\bad-dep.json" $unsafe
    $blocked=@{};foreach($key in $dependency.Keys){$blocked[$key]=$dependency[$key]};$blocked.depends='bad-dep'
    Write-JSON "$work\scoop\buckets\fixture\bucket\blocked.json" $blocked
    $null=Invoke-PSP $centralExe @('install','blocked') 'dependency-hook-rejected' -FailureExpected
    $observations.preflight_before_install_mutation=(-not(Test-Path "$work\scoop\apps\blocked"))-and(-not(Test-Path "$work\scoop\apps\bad-dep"))
    $installedManifest="$personal\scoop\apps\tool\1.5\scoop-manifest.json"
    $originalManifest=[IO.File]::ReadAllBytes($installedManifest)
    $unsafeInstalled=Get-Content $installedManifest -Raw|ConvertFrom-Json
    $unsafeInstalled|Add-Member -NotePropertyName post_uninstall -NotePropertyValue 'throw "must never run"'
    Write-JSON $installedManifest $unsafeInstalled
    $null=Invoke-PSP $personalExe @('uninstall','tool') 'installed-hook-rejected' -FailureExpected
    [IO.File]::WriteAllBytes($installedManifest,$originalManifest)
    $observations.saved_uninstall_preflight=Test-Path "$personal\scoop\apps\tool\1.5\tools\probe.cmd"
    $systemFile="$personal\scoop\apps\scoop\current\lib\system.ps1"
    $originalSystem=[IO.File]::ReadAllBytes($systemFile)
    [IO.File]::AppendAllText($systemFile,"`n# altered")
    $null=Invoke-PSP $personalExe @('list') 'changed-backend-rejected' -FailureExpected
    [IO.File]::WriteAllBytes($systemFile,$originalSystem)
    $observations.backend_failure_no_system_fallback=-not(Test-Path "$root\ordinary-invoked.txt")
    $repository=Join-Path $root 'bucket source'
    New-Item -ItemType Directory -Path "$repository\bucket" -Force|Out-Null
    Copy-Item -LiteralPath "$work\scoop\buckets\fixture\bucket\tool.json","$work\scoop\buckets\fixture\bucket\dep.json" -Destination "$repository\bucket"
    $emptyGitConfig=Join-Path $root 'empty-git.config';[IO.File]::WriteAllText($emptyGitConfig,'')
    $env:GIT_CONFIG_GLOBAL=$emptyGitConfig;$env:GIT_CONFIG_NOSYSTEM='1'
    & $git -C $repository init -q *> "$results\git-init.log"
    & $git -C $repository add . *> "$results\git-add.log"
    & $git -C $repository -c user.name=Fixture -c user.email=fixture@example.invalid -c commit.gpgsign=false commit -q -m fixture *> "$results\git-commit.log"
    if($LASTEXITCODE-ne 0){throw 'Guest bucket fixture Git initialization failed'}
    $env:GIT_DIR=Join-Path $ordinary 'foreign.git';$env:GIT_WORK_TREE=$ordinary
    $null=Invoke-PSP $centralExe @('bucket','add','git-fixture',$repository) 'bucket-add-local-git'
    $observations.bucket_added=Test-Path "$work\scoop\buckets\git-fixture\.git"
    $null=Invoke-PSP $centralExe @('bucket','remove','git-fixture') 'bucket-remove-local-git'
    $observations.bucket_removed=-not(Test-Path "$work\scoop\buckets\git-fixture")
    $observations.inherited_git_controls_isolated=(-not(Test-Path (Join-Path $ordinary 'foreign.git')))-and($env:GIT_WORK_TREE-ceq$ordinary)
    $available=@('R','S','T','U','V','W','X','Y','Z'|Where-Object {-not(Test-Path ($_+':\'))})
    if($available.Count-lt 2){throw 'Two unused guest drive letters are required for qualification'}
    $first=$available[0];$second=$available[1]
    $vhd=Join-Path $root 'portscoop-test.vhd'
    if(Test-Path $vhd){throw 'VHD fixture already exists'}
    Invoke-Diskpart @(('create vdisk file="'+$vhd+'" maximum=128 type=expandable'),('select vdisk file="'+$vhd+'"'),'attach vdisk','create partition primary','format fs=ntfs quick label=PortscoopTest',('assign letter='+$first)) 'create-test-volume'
    if(-not(Test-Path ($first+':\'))){throw 'Guest test volume was not mounted'}
    $driveRoot=$first+':\portable (drive)'
    $null=Invoke-PSP $centralExe @('env','init',$driveRoot,'--name','drive') 'init-drive'
    New-Item -ItemType Directory -Path "$driveRoot\scoop\buckets\fixture\bucket" -Force|Out-Null
    Copy-Item -LiteralPath "$work\scoop\buckets\fixture\bucket\tool.json","$work\scoop\buckets\fixture\bucket\dep.json" -Destination "$driveRoot\scoop\buckets\fixture\bucket"
    $null=Invoke-PSP "$driveRoot\psp.exe" @('install','tool') 'install-drive'
    $null=Invoke-PSP $centralExe @('env','register',$driveRoot,'--name','drive') 'register-drive'
    Invoke-Diskpart @(('select vdisk file="'+$vhd+'"'),'select partition 1',('remove letter='+$first)) 'disconnect-test-volume'
    $null=Invoke-PSP $centralExe @('--env','drive','list') 'disconnected-target-rejected' -FailureExpected
    Invoke-Diskpart @(('select vdisk file="'+$vhd+'"'),'select partition 1',('assign letter='+$second)) 'reassign-test-volume'
    $newDriveRoot=$second+':\portable (drive)'
    $null=Invoke-PSP $centralExe @('--env','drive','list') 'delegated-after-drive-change'
    $driveShim=Get-Content "$newDriveRoot\scoop\shims\fixture-tool.cmd" -Raw
    $observations.drive_change_repaired=$driveShim.Contains($newDriveRoot)-and-not$driveShim.Contains($driveRoot)
    $registry=Get-Content (Join-Path $env:LOCALAPPDATA 'portscoop\registry.json') -Raw|ConvertFrom-Json
    $observations.drive_hint_refreshed=$registry.entries.drive.path -eq $newDriveRoot
    $null=Invoke-PSP $centralExe @('env','unregister','work') 'unregister-preserves-data'
    $observations.unregistration_preserves_data=(Test-Path "$work\.portscoop\environment.json")-and(Test-Path "$work\scoop\persist\tool\data\seed.txt")
    $null=Invoke-PSP "$work\psp.exe" @('list') 'direct-after-unregister'
    foreach($test in @(Get-ChildItem -LiteralPath $root -Filter '*.test.exe')){
        $null=Invoke-PSP $test.FullName @('-test.v') ('suite-'+$test.BaseName)
        $observations[('suite_'+$test.BaseName)]=$true
    }
    $env:PATH=$originalProcessPath;$env:SCOOP=$originalScoop;$env:SCOOP_GLOBAL=$originalGlobal;$env:SCOOP_CACHE=$originalCache
    . (Join-Path $root 'host-state.ps1') | ConvertTo-Json -Depth 6 | Set-Content "$results\host-after.json" -Encoding UTF8
    $observations.host_state_unchanged=(Get-Content "$results\host-before.json" -Raw)-ceq(Get-Content "$results\host-after.json" -Raw)
    foreach($key in @($observations.Keys)){if($observations[$key]-is[bool]-and-not$observations[$key]){throw "Observation failed: $key"}}
    $observations.status='passed'
}catch{
    $observations.status='failed';$observations.error=$_.Exception.Message
    $_ | Out-String | Set-Content "$results\failure.txt" -Encoding UTF8
}finally{
    [Environment]::SetEnvironmentVariable('PATH',$originalUserPath,'User')
    if($server-and-not$server.HasExited){Stop-Process -Id $server.Id -ErrorAction SilentlyContinue}
    foreach($letter in $driveLetters){& "$env:SystemRoot\System32\subst.exe" ($letter+':') /d}
    if($vhd-and(Test-Path $vhd)){try{Invoke-Diskpart @(('select vdisk file="'+$vhd+'"'),'detach vdisk') 'detach-test-volume'}catch{$_|Out-String|Set-Content "$results\volume-cleanup-error.txt"}}
    Write-JSON "$results\observations.json" $observations
    Compress-Archive -Path "$results\*" -DestinationPath "$root\results.zip" -Force
}
if($observations.status-ne'passed'){exit 1}
