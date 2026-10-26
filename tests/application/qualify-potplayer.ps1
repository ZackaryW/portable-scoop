#Requires -Version 5.1
# Disposable VM only. Uses unchanged pinned manifests and digest-checked cache
# payloads, leaves its Stores and phase report available for inspection.
param([Parameter(Mandatory=$true)][string]$Root,[Parameter(Mandatory=$true)][string]$Inputs,[string]$ArchiveName='portable-hooks-dev1.zip',[switch]$InstallOnly,[switch]$ResumeRelocation)
$ErrorActionPreference='Stop'
if ((Get-CimInstance Win32_ComputerSystem).Manufacturer -notmatch 'VMware') { throw 'Disposable VMware guest required' }
$Root=[IO.Path]::GetFullPath($Root)
if((Test-Path -LiteralPath $Root) -and -not $ResumeRelocation){throw 'Fresh qualification root required'}
if(-not $ResumeRelocation){New-Item -ItemType Directory -Path $Root | Out-Null}
Start-Transcript -Path (Join-Path $Root $(if($ResumeRelocation){'relocation.log'}else{'qualification.log'})) | Out-Null
$r=[ordered]@{passed=$false;phase='setup';checks=@();current_revision='6c4f7709ed26a7979bad9e48f73ce549054a45b6';previous_revision='9b92cefb9f3a9a0f0bc2717ab2c1f8d94758050f';error=$null}
$saved=@{};foreach($n in @('PATH','LOCALAPPDATA','GIT_CONFIG_GLOBAL','GIT_CONFIG_NOSYSTEM')){$saved[$n]=[Environment]::GetEnvironmentVariable($n,'Process')}
$persistent=@{};foreach($s in @('User','Machine')){foreach($n in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){$persistent[$s+'/'+$n]=[Environment]::GetEnvironmentVariable($n,$s)}}
function Assert($v,$m){if(-not $v){throw $m}}
function Hash($p){if(Test-Path -LiteralPath $p -PathType Leaf){return (Get-FileHash -LiteralPath $p).Hash};return 'absent'}
function Phase($p){$r.phase=$p;[IO.File]::WriteAllText((Join-Path $Root 'result.json'),($r|ConvertTo-Json -Depth 8))}
function PSP([string]$Exe,[string[]]$CommandArgs){& $Exe @CommandArgs;if($LASTEXITCODE -ne 0){throw "psp failed: $CommandArgs"}}
function Seed($Store,$Manifest,$Payload){
 $m=Get-Content -LiteralPath $Manifest -Raw|ConvertFrom-Json
 Assert ((Hash $Payload) -ieq $m.architecture.'64bit'.hash) 'Payload digest differs from pinned manifest'
 $cache=Join-Path $Store 'scoop\cache';New-Item -ItemType Directory -Path $cache -Force|Out-Null
 $name='potplayer#'+$m.version+'#'+($m.architecture.'64bit'.url -replace '[^\w\.\-]+','_')
 Copy-Item -LiteralPath $Payload -Destination (Join-Path $cache $name)
 Copy-Item -LiteralPath $Manifest -Destination (Join-Path $Store 'scoop\buckets\extras\bucket\potplayer.json')
}
function Settings($Store){
 $persist=Join-Path $Store 'scoop\persist\potplayer'
 foreach($name in @('PotPlayer.ini','PotPlayer64.ini','PotPlayerMini.ini','PotPlayerMini64.ini')){[IO.File]::WriteAllText((Join-Path $persist $name),"[Settings]`r`nPortscoopSentinel=keep`r`n")}
 [IO.File]::WriteAllText((Join-Path $persist 'Logos\user-sentinel.txt'),'keep unrelated resource')
 [IO.File]::WriteAllText((Join-Path $persist 'Logos\PotPlayer.png'),'replace this resource collision')
}
function CheckSettings($Store,$ExpectedLogo){
 $persist=Join-Path $Store 'scoop\persist\potplayer'
 foreach($name in @('PotPlayer.ini','PotPlayer64.ini','PotPlayerMini.ini','PotPlayerMini64.ini')){Assert ([IO.File]::ReadAllText((Join-Path $persist $name)) -ceq "[Settings]`r`nPortscoopSentinel=keep`r`n") "Changed settings $name"}
 Assert ([IO.File]::ReadAllText((Join-Path $persist 'Logos\user-sentinel.txt')) -ceq 'keep unrelated resource') 'Unrelated resource lost'
 Assert ((Hash (Join-Path $persist 'Logos\PotPlayer.png')) -ceq $ExpectedLogo) 'Packaged collision did not replace old resource'
}
function Launch($Store){
 & (Join-Path $Store 'pspe.exe') 'PotPlayer-Mini'
 Assert ($LASTEXITCODE -eq 0) 'Detached shim launch failed'
 $running=@();$deadline=[DateTime]::UtcNow.AddSeconds(15)
 do {
  Start-Sleep -Milliseconds 500
  $running=@(Get-CimInstance Win32_Process|Where-Object{$_.Name -ieq 'PotPlayerMini64.exe' -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($Store+'\',[StringComparison]::OrdinalIgnoreCase)})
 } while($running.Count -eq 0 -and [DateTime]::UtcNow -lt $deadline)
 Assert ($running.Count -gt 0) 'No Store-scoped PotPlayer process appeared'
 foreach($p in $running){Stop-Process -Id $p.ProcessId -ErrorAction Stop}
}
function CheckHost {
 foreach($s in @('User','Machine')){foreach($n in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){Assert ([Environment]::GetEnvironmentVariable($n,$s) -ceq $persistent[$s+'/'+$n]) "Persistent environment changed: $s/$n"}}
 foreach($p in $sentinels.Keys){Assert ((Hash $p) -ceq $sentinels[$p]) "Ordinary host sentinel changed: $p"}
}
function Relocate($Store,$Second,$Moved,$Logo){
 $receipt=Hash (Join-Path $Second '.portscoop\state\runtime.json')
 Move-Item -LiteralPath $Store -Destination $Moved
 PSP (Join-Path $Moved 'psp.exe') @('reset','potplayer');CheckSettings $Moved $Logo;Launch $Moved
 Assert (-not(Test-Path -LiteralPath $Store)) 'Relocation wrote into previous root'
 Assert ((Hash (Join-Path $Second '.portscoop\state\runtime.json')) -ceq $receipt) 'Other runtime changed during relocation'
 CheckSettings $Second $Logo
}
$sentinels=@{};foreach($p in @((Join-Path $env:USERPROFILE '.gitconfig'),(Join-Path $env:USERPROFILE '.config\scoop\config.json'),(Join-Path $env:USERPROFILE 'scoop\config.json'))){$sentinels[$p]=Hash $p}
try {
 $env:PATH=Join-Path $env:SystemRoot 'System32';$env:LOCALAPPDATA=Join-Path $Root 'profile';$env:GIT_CONFIG_GLOBAL='NUL';$env:GIT_CONFIG_NOSYSTEM='1'
 if($ResumeRelocation){
  $prior=Get-Content -LiteralPath (Join-Path $Root 'result.json') -Raw|ConvertFrom-Json
  Assert ($prior.checks -contains 'Real 260819 to 261021 transition preserved settings and refreshed resources; first Store unchanged') 'Prior lifecycle evidence missing'
  $r.checks=@($prior.checks);$r.prior_runtime=$prior.runtime
  $store=Join-Path $Root 'moved store';$second=Join-Path $Root 'second'
  $r.runtime=(& (Join-Path $store 'psp.exe') --version|Out-String).Trim()
  $logo=Hash (Join-Path $second 'scoop\persist\potplayer\Logos\PotPlayer.png')
  Phase 'retry relocation';Relocate $store $second (Join-Path $Root 'relocated store') $logo
  Assert (-not(Test-Path -LiteralPath (Join-Path $Root 'host\store'))) 'Original root recreated'
  CheckHost;$r.checks+='Relocated Store reset and launched; both prior roots absent, second Store and host sentinels preserved'
  $r.passed=$true;Phase 'complete';return
 }
 $payload=Join-Path $Root 'payload';Expand-Archive -LiteralPath (Join-Path $Inputs $ArchiveName) -DestinationPath $payload
 $hostRoot=Join-Path $Root 'host'
 & (Join-Path $payload 'setup.ps1') -Mode Central -Destination $hostRoot -CreateDefaultStore
 Assert ($LASTEXITCODE -eq 0) 'Fresh Host/default Store failed'
 $store=Join-Path $hostRoot 'store';$psp=Join-Path $hostRoot 'psp.exe'
 $r.runtime=(& $psp --version|Out-String).Trim()
 PSP $psp @('env','bootstrap');PSP $psp @('bucket','add','extras')
 $current=Join-Path $Inputs 'potplayer-261021.json';$previous=Join-Path $Inputs 'potplayer-260819.json'
 Seed $store $current (Join-Path $Inputs 'potplayer-261021.exe')
 $r.payload_sha256=(Get-Content $current -Raw|ConvertFrom-Json).architecture.'64bit'.hash
 Phase 'install current';PSP $psp @('install','extras/potplayer')
 Assert (Test-Path -LiteralPath (Join-Path $store 'scoop\apps\potplayer\261021\PotPlayerMini64.exe')) 'Application missing'
 Assert (@(Get-ChildItem -LiteralPath (Join-Path $store 'scoop\apps\potplayer\261021') -Force|Where-Object{$_.Name -like '$*' -or $_.Name -like 'Uninst*'}).Count -eq 0) 'Installer debris retained'
 Launch $store
 $r.checks+='Fresh Host/default Store, Main, private tools, Extras, unchanged current manifest, install and shim launch'
 CheckHost
 if($InstallOnly){$r.checks+='Ordinary Scoop/Git sentinels and persistent environment unchanged';$r.passed=$true;Phase 'complete';return}
 $logo=Hash (Join-Path $store 'scoop\persist\potplayer\Logos\PotPlayer.png')
 Settings $store
 Phase 'reinstall current';PSP $psp @('update','potplayer','--force');CheckSettings $store $logo
 $r.checks+='Same-version reinstall preserved all INIs and unrelated resources; packaged collision refreshed'
 Phase 'second Store and previous version'
 $second=Join-Path $Root 'second';PSP $psp @('env','init',$second,'--name','Second')
 $secondPSP=Join-Path $second 'psp.exe';PSP $secondPSP @('bucket','add','extras')
 Seed $second $previous (Join-Path $Inputs 'potplayer-260819.exe')
 PSP $secondPSP @('install','extras/potplayer');Settings $second
 Seed $second $current (Join-Path $Inputs 'potplayer-261021.exe')
 Phase 'real version transition 260819 to 261021';PSP $secondPSP @('update','potplayer');CheckSettings $second $logo
 Assert (Test-Path -LiteralPath (Join-Path $second 'scoop\apps\potplayer\261021\PotPlayerMini64.exe')) 'New version missing'
 CheckSettings $store $logo
 $r.checks+='Real 260819 to 261021 transition preserved settings and refreshed resources; first Store unchanged'
 Phase 'relocation';Relocate $store $second (Join-Path $Root 'moved store') $logo
 $r.checks+='Relocated Store repaired links and launched; prior root absent and second Store preserved'
 CheckHost
 $r.checks+='Ordinary Scoop/Git sentinels and persistent environment unchanged';$r.passed=$true;Phase 'complete'
} catch {$r.error=$_.Exception.Message;Phase 'failed';throw} finally {
 foreach($n in $saved.Keys){[Environment]::SetEnvironmentVariable($n,$saved[$n],'Process')}
 Stop-Transcript|Out-Null
}
