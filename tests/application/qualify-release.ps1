#Requires -Version 5.1
# Run only in the explicitly selected disposable VMware guest.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Bundle,
    [Parameter(Mandatory=$true)][string]$Results,
    [Parameter(Mandatory=$true)][string]$LegacyRelease
)
$ErrorActionPreference='Stop'
$machine=Get-CimInstance Win32_ComputerSystem
if($machine.Manufacturer-notmatch'VMware'-and$machine.Model-notmatch'VMware'){throw 'Disposable VMware guest required'}
$identity=[Security.Principal.WindowsIdentity]::GetCurrent()
try{$principal=[Security.Principal.WindowsPrincipal]::new($identity);if(-not$principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){throw 'Run from an elevated PowerShell session on the interactive guest desktop; owned VHD checks require administrator privileges'}}finally{$identity.Dispose()}
$LegacyRelease=[IO.Path]::GetFullPath($LegacyRelease)
foreach($name in @('portscoop.exe','psp.exe')){if(-not(Test-Path -LiteralPath (Join-Path $LegacyRelease $name) -PathType Leaf)){throw ('Compatible older release is missing '+$name+': '+$LegacyRelease)}}
$root=Join-Path 'C:\Users\Public' ('PortscoopExecution-'+[guid]::NewGuid().ToString('N'))
$keys=@('PATH','LOCALAPPDATA','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE','GIT_CONFIG_GLOBAL','GIT_CONFIG_NOSYSTEM','GIT_SSH_COMMAND','PSPE_FIXTURE_MODE','PSPE_FIXTURE_OUTPUT','PSPE_FIXTURE_EXIT')
$original=@{};foreach($key in $keys){$original[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$status=[ordered]@{schema_version=1;status='running';root=$root;windows=[Environment]::OSVersion.Version.ToString();powershell=$PSVersionTable.PSVersion.ToString();go_absent=(-not[bool](Get-Command go -ErrorAction SilentlyContinue));checks=@();observations=@{};error=$null}
$ownedProcesses=New-Object 'Collections.Generic.List[int]'
$ownedProcessInfo=@{};$ownedPowerShell=New-Object 'Collections.Generic.HashSet[int]'
$server=$null;$vhd=$null;$before=$null;$sequence=0
function Assert([bool]$Condition,[string]$Message){if(-not$Condition){throw $Message}}
function Write-JSON([string]$Path,$Value){[IO.File]::WriteAllText($Path,($Value|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))}
function Invoke-Checked([string]$Executable,[string[]]$ArgVector,[int]$Expected=0,[string]$InputText=$null){
    $script:sequence++;$log=Join-Path $root ('command-{0:d3}.log'-f$script:sequence)
    $previous=$ErrorActionPreference;$ErrorActionPreference='Continue'
    try{if($null-ne$InputText){$output=$InputText|& $Executable @ArgVector 2>&1|Out-String}else{$output=& $Executable @ArgVector 2>&1|Out-String};$code=$LASTEXITCODE}finally{$ErrorActionPreference=$previous}
    [IO.File]::WriteAllText($log,$output,[Text.UTF8Encoding]::new($false))
    Assert ($code-eq$Expected) ('Command failed: '+$Executable+' status='+$code+' expected='+$Expected+' log='+$log+' output='+$output)
    return $output
}
function Output-Path([string]$Name){$path=Join-Path $root ($Name+'.json');$env:PSPE_FIXTURE_OUTPUT=$path;return $path}
function Read-Observation([string]$Path){
    $deadline=[DateTime]::UtcNow.AddSeconds(8)
    while([DateTime]::UtcNow-lt$deadline){if(Test-Path -LiteralPath $Path){try{return ([IO.File]::ReadAllText($Path,[Text.Encoding]::UTF8)|ConvertFrom-Json)}catch{}};Start-Sleep -Milliseconds 20}
    throw ('Application did not record: '+$Path)
}
function Assert-Args($Actual,[string[]]$Wanted){Assert (@($Actual).Count-eq$Wanted.Count) 'Argument count changed';for($i=0;$i-lt$Wanted.Count;$i++){Assert ([string]$Actual[$i]-ceq$Wanted[$i]) ('Argument '+$i+' changed')}}
function Keep-Process([int]$AppId){$ownedProcesses.Add($AppId);$p=Get-Process -Id $AppId -ErrorAction SilentlyContinue;Assert ([bool]$p) 'Acknowledged application is not running';$ownedProcessInfo[$AppId]=@{image=$p.Path;started=$p.StartTime.Ticks}}
function Stop-Owned([int]$AppId){
    $process=Get-Process -Id $AppId -ErrorAction SilentlyContinue
    if($process){$image=$process.Path;$known=$ownedProcessInfo[$AppId];if(-not$known-or$known.started-ne$process.StartTime.Ticks){return};Assert ($image-and($image.StartsWith($root+'\',[StringComparison]::OrdinalIgnoreCase)-or$image-match'^[R-Z]:\\portable-drive\\'-or($ownedPowerShell.Contains($AppId)-and$image-ieq("$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe")))) 'Refusing to stop a process outside fixture roots';Stop-Process -Id $AppId -Force;$process.WaitForExit(5000)|Out-Null}
}
function Visible-Consoles{ $all=(& $fixture --fixture-windows 0|Out-String)|ConvertFrom-Json;return @($all|Where-Object {$_.Visible-and$_.Class-eq'ConsoleWindowClass'}|ForEach-Object {[string]$_.Handle}) }
function Assert-NoHelperConsole{ $after=@(Visible-Consoles);foreach($handle in $after){Assert ($consoleBaseline-contains$handle) 'A launch helper displayed an extra console'} }
function Invoke-Diskpart([string[]]$Lines,[string]$Name){$file=Join-Path $root ($Name+'.txt');[IO.File]::WriteAllLines($file,$Lines,[Text.Encoding]::ASCII);& "$env:SystemRoot\System32\diskpart.exe" /s $file|Out-String|Set-Content (Join-Path $root ($Name+'.log'));Assert ($LASTEXITCODE-eq 0) ('Diskpart fixture failed: '+$Name)}
try{
    Expand-Archive -LiteralPath $Bundle -DestinationPath $root
    Start-Transcript -Path (Join-Path $root 'execution.log')|Out-Null
    $before=& (Join-Path $root 'host-state.ps1');Write-JSON (Join-Path $root 'host-before.json') $before
    $status.release_sha256=(Get-FileHash (Join-Path $root 'portscoop-windows-amd64.zip') -Algorithm SHA256).Hash.ToLowerInvariant()
    $release=Join-Path $root 'release';Expand-Archive -LiteralPath (Join-Path $root 'portscoop-windows-amd64.zip') -DestinationPath $release
    $fixture=Join-Path $root 'pspe-fixture.exe'
    $jobContext=(& $fixture --fixture-job-probe (Join-Path $release 'pspe.exe')|Out-String)|ConvertFrom-Json
    $status.observations.guest_external_job_after_breakaway=$jobContext
    Assert (-not$jobContext.in_job) 'Run this success qualification outside an owning job on the interactive guest desktop'
    $env:LOCALAPPDATA=Join-Path $root 'host-profile';New-Item -ItemType Directory -Path $env:LOCALAPPDATA|Out-Null
    $hostRoot=Join-Path $root 'Host';$hostPSP=Join-Path $hostRoot 'psp.exe';$hostPSPE=Join-Path $hostRoot 'pspe.exe'
    & (Join-Path $release 'setup.ps1') -Mode Central -Destination $hostRoot -CreateDefaultStore
    $work=Join-Path $hostRoot 'store';$personal=Join-Path $root 'Personal Store'
    $null=Invoke-Checked $hostPSP @('env','init',$personal,'--name','Personal')
    $null=Invoke-Checked $hostPSP @('env','register',$work,'--name','work')
    $null=Invoke-Checked $hostPSP @('env','register',$personal,'--name','personal')
    $workID=(Get-Content (Join-Path $work '.portscoop\environment.json') -Raw|ConvertFrom-Json).id
    $personalID=(Get-Content (Join-Path $personal '.portscoop\environment.json') -Raw|ConvertFrom-Json).id
    Assert ($workID-ne$personalID) 'Stores share an identity'
    foreach($deployment in @($hostRoot,$work,$personal)){foreach($name in @('portscoop.exe','psp.exe','pspe.exe')){Assert (Test-Path (Join-Path $deployment $name)) 'Deployment lacks a launcher'}}
    $status.checks+='Verified release creates Host, default Store, and another independent Store with all three launchers'

    $contents=Join-Path $root 'contents';$payload=Join-Path $root 'payload';New-Item -ItemType Directory -Path "$contents\tools","$contents\data",$payload|Out-Null
    Copy-Item (Join-Path $root 'pspe-fixture.exe') "$contents\tools\console.exe"
    Copy-Item (Join-Path $root 'pspe-gui-fixture.exe') "$contents\tools\gui.exe"
    $psSource=@'
param([string]$Name)
$result=@{name=$Name;args=@($args);cwd=(Get-Location).ProviderPath;root=$PSScriptRoot;pid=$PID;home=$env:PSPE_STORE_HOME}
[IO.File]::WriteAllText($env:PSPE_FIXTURE_OUTPUT,($result|ConvertTo-Json -Depth 5),[Text.UTF8Encoding]::new($false))
[Console]::Error.WriteLine('script stderr')
if($env:PSPE_FIXTURE_MODE-eq'linger'){Start-Sleep -Seconds 30}
exit 37
'@
    [IO.File]::WriteAllText("$contents\tools\script.ps1",$psSource,[Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText("$contents\tools\batch.cmd",'@"%~dp0console.exe" %*'+"`r`n@exit /b %errorlevel%`r`n",[Text.Encoding]::ASCII)
    [IO.File]::WriteAllText("$contents\data\user.txt",'preserved')
    Compress-Archive -Path "$contents\*" -DestinationPath "$payload\package.zip"
    $ready=Join-Path $root 'server-ready.json'
    $server=Start-Process -FilePath (Join-Path $root 'probe-server.exe') -ArgumentList @('-root',('"'+$payload+'"'),'-ready-file',('"'+$ready+'"')) -PassThru -WindowStyle Hidden
    $url=(Read-Observation $ready).url;$hash=(Get-FileHash "$payload\package.zip").Hash.ToLowerInvariant()
    $manifest=@{version='1.0';url="$url/package.zip";hash=$hash;bin=@(@('tools\console.exe','console'),@('tools\gui.exe','gui'),@('tools\script.ps1','script'),@('tools\batch.cmd','batch'));persist='data';env_set=@{PSPE_STORE_HOME='$dir'}}
    foreach($storeRoot in @($work,$personal)){New-Item -ItemType Directory -Path "$storeRoot\scoop\buckets\fixture\bucket" -Force|Out-Null;Write-JSON "$storeRoot\scoop\buckets\fixture\bucket\probe.json" $manifest;$null=Invoke-Checked (Join-Path $storeRoot 'psp.exe') @('install','fixture/probe')}
    $status.checks+='Real pinned backend installs owned hashed ZIP fixtures into both Stores'

    $fixture=Join-Path $root 'pspe-fixture.exe';$env:PSPE_FIXTURE_MODE='record'
    $consoleBaseline=@(Visible-Consoles)
    $env:GIT_CONFIG_GLOBAL=Join-Path $root 'caller.gitconfig';[IO.File]::WriteAllText($env:GIT_CONFIG_GLOBAL,'')
    $env:GIT_SSH_COMMAND='caller-ssh';$env:GIT_CONFIG_NOSYSTEM='1'
    $foreign=Join-Path $root 'ordinary-scoop-sentinel';New-Item -ItemType Directory -Path "$foreign\shims" -Force|Out-Null
    Copy-Item (Join-Path $root 'pspe-fixture.exe') "$foreign\shims\missing.exe"
    $env:SCOOP=$foreign;$env:SCOOP_GLOBAL="$foreign\global";$env:SCOOP_CACHE="$foreign\cache";$env:PATH="$foreign\shims;"+$env:PATH
    Push-Location -LiteralPath $root
    try{
        foreach($storeCase in @(@((Join-Path $work 'pspe.exe'),@()),@($hostPSPE,@('--env','personal')))){
            $path=Output-Path ('native-'+[guid]::NewGuid().ToString('N'))
            $appArgs=@('--key=value','space value',([string][char]0x96ea),'--env','application','--help')
            $null=Invoke-Checked $storeCase[0] ($storeCase[1]+@('console')+$appArgs) 37 'shell input'
            $o=Read-Observation $path;Assert-Args $o.args $appArgs
            Assert ($o.Cwd-ceq$root-and$o.Input-match'shell input') 'Native working directory or stdin changed'
            Assert ($o.Environment.GIT_CONFIG_GLOBAL-ceq$env:GIT_CONFIG_GLOBAL-and$o.Environment.GIT_SSH_COMMAND-ceq'caller-ssh') 'Application inherited backend Git restrictions'
            Assert ($o.Environment.SCOOP-ne$foreign) 'Application inherited ordinary Scoop root'
        }
        $path=Output-Path 'cmd-shell';$cmd=Join-Path $root 'caller.cmd'
        [IO.File]::WriteAllText($cmd,('@echo cmd-input | "'+$hostPSPE+'" --env work console --key=value "" "space value" > "'+$root+'\cmd-output.log" 2>&1'+"`r`n@exit /b %errorlevel%"),[Text.Encoding]::ASCII)
        $null=Invoke-Checked "$env:SystemRoot\System32\cmd.exe" @('/d','/c',$cmd) 37
        $o=Read-Observation $path;Assert-Args $o.args @('--key=value','','space value');Assert ($o.Input-match'cmd-input') 'CMD stdin changed'
        $path=Output-Path 'missing-shim';$text=Invoke-Checked $hostPSPE @('--env','work','missing') 2
        Assert ($text-match'not installed'-and-not(Test-Path $path)) 'Missing shim used host PATH fallback'
        $status.checks+='CMD and PowerShell preserve native mode, named tail, caller cwd, input, app status, Store environment, Git settings, and no outside fallback'

        $path=Output-Path 'script-wait';$null=Invoke-Checked $hostPSPE @('--env','work','script','-Name',([string][char]0x96ea),'--key=value','space value') 37
        $o=Read-Observation $path;Assert ($o.name-ceq([string][char]0x96ea)) 'PowerShell named parameter changed';Assert-Args $o.args @('--key=value','space value')
        $path=Output-Path 'script-detach';$null=Invoke-Checked (Join-Path $personal 'pspe.exe') @('--detach','script','-Name','detached','--help')
        $o=Read-Observation $path;Assert ($o.name-ceq'detached') 'Detached script did not consume payload'
        $path=Output-Path 'batch-wait';$null=Invoke-Checked $hostPSPE @('--env','work','batch','%PATH%','!data!','&|<>','--key=value') 37 'batch input'
        $o=Read-Observation $path;Assert-Args $o.args @('%PATH%','!data!','&|<>','--key=value');Assert ($o.Input-match'batch input') 'Batch input changed'
        $path=Output-Path 'batch-rejected';$quoteCase=Join-Path $root 'quote-case.json'
        Write-JSON $quoteCase @{Executable=$hostPSPE;Args=@('--env','work','batch','embedded"quote');Directory=$root;AllowBreakaway=$true}
        $quoteResult=(Invoke-Checked $fixture @('--fixture-job',$quoteCase))|ConvertFrom-Json
        Assert ($quoteResult.code-eq2-and$quoteResult.output-match'not qualified'-and-not(Test-Path $path)) 'Unqualified batch quote form was launched'
        $status.checks+='Advertised PowerShell and batch handlers pass named/Unicode/metacharacter and both-mode startup checks; unqualified batch quotes fail explicitly'

        $env:PSPE_FIXTURE_MODE='linger';$path=Output-Path 'bare-detach';$null=Invoke-Checked $hostPSPE @('--env','work','console')
        $o=Read-Observation $path;Keep-Process $o.PID
        Assert ($o.HandleTypes.stdin-ne3-and$o.HandleTypes.stdout-ne3-and$o.HandleTypes.stderr-ne3) 'Detached app retained a terminal pipe'
        $null=Invoke-Checked $hostPSP @('--env','work','list')
        Stop-Owned $o.PID
        $path=Output-Path 'script-hidden-console';$null=Invoke-Checked $hostPSPE @('--env','work','--detach','script','-Name','hidden console')
        $o=Read-Observation $path;Assert ($o.root.StartsWith($work+'\')) 'Script readiness came from outside its package';$null=$ownedPowerShell.Add([int]$o.pid);Keep-Process $o.pid;Assert-NoHelperConsole;Stop-Owned $o.pid
        $path=Output-Path 'batch-detach';$null=Invoke-Checked (Join-Path $work 'pspe.exe') @('--detach','batch','%PATH%','!data!')
        $o=Read-Observation $path;Keep-Process $o.PID;Assert-Args $o.args @('%PATH%','!data!');Stop-Owned $o.PID
        Assert-NoHelperConsole
        $env:PSPE_FIXTURE_MODE='launcher';$path=Output-Path 'foreground-launcher'
        $null=Invoke-Checked $hostPSPE @('--env','work','--wait','console') 23
        $o=Read-Observation $path;Keep-Process $o.PID;$null=Invoke-Checked $hostPSP @('--env','work','list');Stop-Owned $o.PID
        $status.checks+='Detached apps survive helper completion with NUL streams; foreground launcher children survive; live apps do not retain the package lock'

        $env:PSPE_FIXTURE_MODE='interactive'
        foreach($central in @($false,$true)){foreach($ctrlC in @($false,$true)){
            $caseName='pty-'+$central+'-'+$ctrlC;$config=Join-Path $root ($caseName+'.json')
            $executable=Join-Path $work 'pspe.exe';$argv=@('--wait','console')
            if($central){$executable=$hostPSPE;$argv=@('--env','work')+$argv}
            Write-JSON $config @{Executable=$executable;Args=$argv;Directory=$root;Cancel=$ctrlC}
            $raw=Invoke-Checked $fixture @('--fixture-pty',$config);$pty=$raw|ConvertFrom-Json
            Assert ($pty.console-and$pty.output-match'PSPE_READY console=true'-and$pty.output-match'PSPE_INPUT=terminal-input') 'ConPTY did not prove interactive console input'
            if($ctrlC){Assert ($pty.code-eq130-and$pty.output-match'pspe: application execution cancelled') 'Ctrl+C status/diagnostic failed'}else{Assert ($pty.code-eq0) 'Interactive normal exit failed'}
            $status.observations[$caseName]=$pty
        }}
        $status.checks+='Real Windows ConPTY: direct and Host --wait with no app arguments receive console input; Ctrl+C returns 130 and a cancellation diagnostic'

        $env:PSPE_FIXTURE_MODE='gui'
        foreach($central in @($false,$true)){
            $path=Output-Path ('gui-'+$central);$executable=Join-Path $work 'pspe.exe';$argv=@('gui');if($central){$executable=$hostPSPE;$argv=@('--env','personal','gui')}
            $null=Invoke-Checked $executable $argv;$o=Read-Observation $path;Keep-Process $o.PID
            $deadline=[DateTime]::UtcNow.AddSeconds(5);$visible=$false
            while([DateTime]::UtcNow-lt$deadline){$windows=(& $fixture --fixture-windows ([string]$o.PID)|Out-String)|ConvertFrom-Json;$visible=[bool]@($windows|Where-Object {$_.Visible-and$_.Title-match'PSPE GUI fixture'}).Count;if($visible){break};Start-Sleep -Milliseconds 50}
            Assert $visible 'Detached GUI window was hidden';Assert-NoHelperConsole;$status.observations[('gui-'+$central)]=$windows;Stop-Owned $o.PID
        }
        foreach($target in @('console','gui')){foreach($allow in @($false,$true)){
            $env:PSPE_FIXTURE_MODE=if($target-eq'gui'){'gui'}else{'linger'}
            $path=Output-Path ('job-'+$target+'-'+$allow);$jobDir=Join-Path $root ('job-'+$target+'-'+$allow);New-Item -ItemType Directory -Path $jobDir|Out-Null
            $config=Join-Path $jobDir 'config.json';Write-JSON $config @{Executable=$hostPSPE;Args=@('--env','work',$target);Directory=$jobDir;AllowBreakaway=$allow}
            $job=(Invoke-Checked $fixture @('--fixture-job',$config))|ConvertFrom-Json
            if($allow){Assert ($job.code-eq0) 'Breakaway-enabled nested job launch failed';$o=Read-Observation $path;Keep-Process $o.PID;Stop-Owned $o.PID}else{Assert ($job.code-eq2-and$job.output-match'external job'-and-not(Test-Path $path)) 'Restrictive external job did not fail clearly before app creation'}
            $status.observations[('job-'+$target+'-'+$allow)]=$job
        }}
        $status.checks+='Direct/Host GUI windows are visible; console and GUI apps survive permitted nested jobs; restrictive external jobs produce explicit launch diagnostics'

        $env:PSPE_FIXTURE_MODE='record';$moved=Join-Path $root 'Moved Work Store'
        Assert ([IO.Path]::GetFullPath($work).StartsWith($root+'\')-and[IO.Path]::GetFullPath($moved).StartsWith($root+'\')) 'Move escaped fixture root'
        Move-Item -LiteralPath $work -Destination $moved;$work=$moved
        $path=Output-Path 'after-directory-move';$null=Invoke-Checked (Join-Path $work 'pspe.exe') @('console','--moved') 37 'moved input'
        $o=Read-Observation $path;Assert ($o.Environment.SCOOP-ceq(Join-Path $work 'scoop')-and$o.Environment.PSPE_STORE_HOME.StartsWith($work+'\')) 'Relocated application state was not applied'
        $null=Invoke-Checked $hostPSP @('env','register',$work,'--name','work')
        $status.checks+='Directory relocation repairs native shim metadata and portable application state while preserving caller directory'

        $available=@('R','S','T','U','V','W','X','Y','Z'|Where-Object {-not(Test-Path ($_+':\'))});Assert ($available.Count-ge2) 'Two unused drive letters required'
        $first=$available[0];$second=$available[1];$vhd=Join-Path $root 'execution-test.vhd'
        Invoke-Diskpart @(('create vdisk file="'+$vhd+'" maximum=256 type=expandable'),('select vdisk file="'+$vhd+'"'),'attach vdisk','create partition primary','format fs=ntfs quick label=PortscoopExecution',('assign letter='+$first)) 'create-owned-volume'
        $driveRoot=$first+':\portable-drive';$null=Invoke-Checked $hostPSP @('env','init',$driveRoot,'--name','Drive')
        New-Item -ItemType Directory -Path "$driveRoot\scoop\buckets\fixture\bucket" -Force|Out-Null;Write-JSON "$driveRoot\scoop\buckets\fixture\bucket\probe.json" $manifest
        $null=Invoke-Checked (Join-Path $driveRoot 'psp.exe') @('install','fixture/probe');$null=Invoke-Checked $hostPSP @('env','register',$driveRoot,'--name','drive')
        Invoke-Diskpart @(('select vdisk file="'+$vhd+'"'),'select partition 1',('remove letter='+$first),('assign letter='+$second)) 'reassign-owned-volume'
        $newDrive=$second+':\portable-drive';$path=Output-Path 'after-drive-change'
        $null=Invoke-Checked $hostPSPE @('--env','drive','console','--drive') 37 'drive input'
        $o=Read-Observation $path;Assert ($o.Environment.SCOOP-ceq($newDrive+'\scoop')-and$o.Environment.PSPE_STORE_HOME.StartsWith($newDrive+'\')) 'Drive rediscovery did not relocate app settings'
        $reg=Get-Content (Join-Path $env:LOCALAPPDATA 'portscoop\registry.json') -Raw|ConvertFrom-Json;Assert ($reg.entries.drive.path-ceq$newDrive) 'Execution did not refresh location hint'
        $status.checks+='Drive-letter reassignment on an owned NTFS VHD rediscovered the same Store, repaired launch state, and refreshed its registry hint'

        $legacy=Join-Path $root 'Legacy Store';$null=Invoke-Checked $hostPSP @('env','init',$legacy,'--name','Legacy');$null=Invoke-Checked $hostPSP @('env','register',$legacy,'--name','legacy')
        $legacyMarker=[IO.File]::ReadAllText((Join-Path $legacy '.portscoop\environment.json'))
        $oldRelease=$LegacyRelease
        foreach($name in @('portscoop.exe','psp.exe')){Copy-Item (Join-Path $oldRelease $name) (Join-Path $legacy $name) -Force}
        Remove-Item -LiteralPath (Join-Path $legacy 'pspe.exe')
        [IO.File]::WriteAllText((Join-Path $legacy 'scoop\persist\user-data.txt'),'preserve')
        $null=Invoke-Checked $hostPSP @('--env','legacy','list') 1
        $text=Invoke-Checked $hostPSPE @('--env','legacy','console') 2;Assert ($text-match'compatible execution support') 'Legacy Store had no upgrade diagnostic'
        foreach($name in @('portscoop.exe','psp.exe','pspe.exe')){Copy-Item (Join-Path $release $name) (Join-Path $legacy $name) -Force}
        $null=Invoke-Checked (Join-Path $legacy 'pspe.exe') @('--version');$null=Invoke-Checked $hostPSP @('--env','legacy','list') 1
        foreach($name in @('portscoop.exe','psp.exe')){Copy-Item (Join-Path $oldRelease $name) (Join-Path $legacy $name) -Force};Remove-Item -LiteralPath (Join-Path $legacy 'pspe.exe')
        $null=Invoke-Checked $hostPSP @('--env','legacy','list') 1
        Assert ([IO.File]::ReadAllText((Join-Path $legacy '.portscoop\environment.json'))-ceq$legacyMarker-and[IO.File]::ReadAllText((Join-Path $legacy 'scoop\persist\user-data.txt'))-ceq'preserve') 'Legacy deployment/rollback changed identity or data'
        $status.checks+='Real older management binaries remain usable across missing execution support, compatible launcher deployment, and rollback with identity/persistence preserved'
    }finally{Pop-Location}

    $env:PATH=(Join-Path $env:ProgramFiles 'Git\cmd')+';'+$original.PATH
    foreach($suite in @('application','runner')){$null=Invoke-Checked (Join-Path $root ($suite+'.test.exe')) @('-test.v','-test.timeout=120s')}
    $status.checks+='Compiled application and unchanged package-runner suites pass inside the no-Go guest, including exact received empty/quote/Unicode vectors and lost/pre-commit acknowledgement cases'
    foreach($script in @('qualify-installer.ps1','qualify-host-bootstrap.ps1')){
        $resultFile=Join-Path $root ($script+'.json')
        $null=Invoke-Checked "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" @('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',(Join-Path $root $script),'-Bundle',$Bundle,'-Results',$resultFile)
        $r=Read-Observation $resultFile;Assert ($r.status-eq'passed') ('Setup qualification failed: '+$r.error);$status.observations[$script]=$r
    }
    $status.checks+='Internet-shaped setup/Host PATH opt-in and first-default-Store qualification pass with all three executables'
    Assert $status.go_absent 'Guest unexpectedly contains Go'
    $status.status='passed'
}catch{$status.status='failed';$status.error=$_.Exception.ToString();$status.failure_position=$_.InvocationInfo.PositionMessage;$status.failure_stack=$_.ScriptStackTrace;$_|Out-String|Set-Content (Join-Path $root 'failure.txt') -Encoding UTF8}
finally{
    foreach($appId in $ownedProcesses){try{Stop-Owned $appId}catch{}}
    if($server-and-not$server.HasExited){Stop-Process -Id $server.Id -Force}
    if($vhd-and(Test-Path $vhd)){try{Invoke-Diskpart @(('select vdisk file="'+$vhd+'"'),'detach vdisk') 'detach-owned-volume'}catch{$status.cleanup_error=$_.Exception.Message;$status.status='failed'}}
    foreach($key in $keys){[Environment]::SetEnvironmentVariable($key,$original[$key],'Process')}
    if($before){$after=& (Join-Path $root 'host-state.ps1');Write-JSON (Join-Path $root 'host-after.json') $after;$same=(($before|ConvertTo-Json -Depth 8 -Compress)-ceq($after|ConvertTo-Json -Depth 8 -Compress));$status.observations.guest_scoop_and_persistent_environment_unchanged=$same;if(-not$same){$status.status='failed';$status.sentinel_error='Guest sentinel changed'}}
    Write-JSON $Results $status;Write-JSON (Join-Path $root 'results.json') $status
    try{Stop-Transcript|Out-Null}catch{}
}
if($status.status-ne'passed'){throw ('Application qualification failed; inspect '+$Results)}
$status|ConvertTo-Json -Depth 30
