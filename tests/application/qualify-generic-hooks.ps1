#Requires -Version 5.1
# Run only in a disposable VMware guest. Leave Store and evidence inspectable.
param([Parameter(Mandatory=$true)][string]$Root)
$ErrorActionPreference='Stop'
if ((Get-CimInstance Win32_ComputerSystem).Manufacturer -notmatch 'VMware') { throw 'VMware qualification only' }
$Root=[IO.Path]::GetFullPath($Root)
$store=Join-Path $Root 'store'
if (Test-Path -LiteralPath $store) { throw 'Fresh Store required' }
Start-Transcript -Path "$Root/qualification.log" | Out-Null
$r=[ordered]@{passed=$false;kind='generic-hook-integration';apps=@();error=$null}
$server=$null
function Assert($condition,$message) { if (-not $condition) { throw $message } }
function Fingerprint($path) {
 if (-not (Test-Path -LiteralPath $path)) { return 'absent' }
 return (@(Get-ChildItem -LiteralPath $path -Recurse -File -Force | Sort-Object FullName | ForEach-Object { $_.FullName.Substring($path.Length)+':'+(Get-FileHash -LiteralPath $_.FullName).Hash }) -join "`n")
}
function PSP([string[]]$arguments) { & "$store/psp.exe" @arguments; if ($LASTEXITCODE -ne 0) { throw "psp failed: $arguments" } }
try {
 $hostProfile=Join-Path $env:APPDATA 'Bitwarden'
 $profileBefore=Fingerprint $hostProfile; $scoopBefore=Fingerprint "$env:USERPROFILE/scoop"
 $userPath=[Environment]::GetEnvironmentVariable('PATH','User'); $machinePath=[Environment]::GetEnvironmentVariable('PATH','Machine')
 Expand-Archive -LiteralPath "$Root/release.zip" -DestinationPath $store
 & "$store/setup.ps1" -Mode Portable -Destination $store -Name generic-hooks
 Assert ($LASTEXITCODE -eq 0) 'Setup failed'
 $bucket="$store/scoop/buckets/extras/bucket"
 New-Item -ItemType Directory -Path $bucket -Force | Out-Null
 Copy-Item -LiteralPath "$Root/bitwarden.json","$Root/spacesniffer.json" -Destination $bucket
 foreach ($name in @('bitwarden','spacesniffer')) {
  PSP @('install',"extras/$name")
  $manifest=Get-Content "$bucket/$name.json" -Raw | ConvertFrom-Json
  $physical="$store/scoop/apps/$name/$($manifest.version)"
  $persist="$store/scoop/persist/$name"
  if ($name -eq 'bitwarden') {
   Assert ((Get-FileHash "$physical/Bitwarden.exe").Hash -ieq $manifest.hash) 'Executable digest mismatch'
   Assert (-not (Test-Path "$physical/Bitwarden-Portable-$($manifest.version).exe")) 'Rename not performed'
   [IO.File]::WriteAllText("$persist/bitwarden-appdata/portscoop-qualification.txt",'preserve me')
  } else { [IO.File]::WriteAllText("$persist/SpaceSnifferConfig.xml",'<configuration/>') }
  $before=Fingerprint $persist
  PSP @('reset',$name)
  Assert ((Fingerprint $persist) -ceq $before) 'Reset changed persisted data'
  & "$store/pspe.exe" $name
  Assert ($LASTEXITCODE -eq 0) 'Detached launch failed'
  Start-Sleep -Seconds 12
  $running=@(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($Root+'\',[StringComparison]::OrdinalIgnoreCase) -and $_.Name -ieq ($name+'.exe') })
  Assert ($running.Count -gt 0) "No root-scoped process for $name"
  if ($name -eq 'bitwarden') { Assert (@(Get-ChildItem "$persist/bitwarden-appdata" -Recurse -File).Count -gt 1) 'Portable profile not created' }
  $r.apps+=@{name=$name;version=$manifest.version;install=$true;reset_preserved=$true;launch=$true;processes=@($running | Select-Object ProcessId,ExecutablePath)}
 }
 # A separate synthetic package proves unrelated operands, both phases, a
 # selective skip plus required renames, and explicit fragment/query naming.
 $payload="$Root/payload"; $contents="$Root/contents"
 New-Item -ItemType Directory -Path $payload,$contents -Force | Out-Null
 [IO.File]::WriteAllText("$contents/first.exe",'inert synthetic payload')
 Compress-Archive -Path "$contents/*" -DestinationPath "$payload/test.zip"
 $ready="$Root/server-ready.json"
 $server=Start-Process -FilePath "$Root/probe-server.exe" -ArgumentList @('-root',('"'+$payload+'"'),'-ready-file',('"'+$ready+'"')) -PassThru -WindowStyle Hidden
 $deadline=[DateTime]::UtcNow.AddSeconds(10)
 while (-not (Test-Path $ready)) { if ($server.HasExited -or [DateTime]::UtcNow -gt $deadline) { throw 'Fixture server failed' }; Start-Sleep -Milliseconds 100 }
 $address=(Get-Content $ready -Raw | ConvertFrom-Json).url
 $synthetic=@{version='1';url="$address/test.zip?name=wrong.exe#/download.zip";hash=(Get-FileHash "$payload/test.zip").Hash.ToLowerInvariant();persist='profile';pre_install=@('Rename-Item "$dir\first.exe" ''second.exe''','if (!(Test-Path "$dir\profile\*") -and (Test-Path "$env:APPDATA\PortscoopSynthetic")) { Copy-Item "$env:APPDATA\PortscoopSynthetic\*" "$dir\profile" -Recurse }');post_install='Rename-Item "$dir\second.exe" ''final.exe'''}
 $synthetic | ConvertTo-Json -Depth 8 | Set-Content "$bucket/aurora.json" -Encoding UTF8
 PSP @('install','extras/aurora')
 Assert ([IO.File]::ReadAllText("$store/scoop/apps/aurora/1/final.exe") -ceq 'inert synthetic payload') 'Synthetic ordered phase actions failed'
 # An all-skip hook receives an explicit empty action plan.
 $synthetic.pre_install=@($synthetic.pre_install[1]); $synthetic.Remove('post_install')
 $synthetic | ConvertTo-Json -Depth 8 | Set-Content "$bucket/nebula.json" -Encoding UTF8
 PSP @('install','extras/nebula')
 Assert ([IO.File]::Exists("$store/scoop/apps/nebula/1/first.exe")) 'All-skip installation failed'
 $r.synthetic=$true
 $r.hostProfileUnchanged=(Fingerprint $hostProfile) -ceq $profileBefore
 $r.ordinaryScoopUnchanged=(Fingerprint "$env:USERPROFILE/scoop") -ceq $scoopBefore
 $r.persistentPathUnchanged=$userPath -ceq [Environment]::GetEnvironmentVariable('PATH','User') -and $machinePath -ceq [Environment]::GetEnvironmentVariable('PATH','Machine')
 Assert ($r.hostProfileUnchanged -and $r.ordinaryScoopUnchanged -and $r.persistentPathUnchanged) 'Guest host state changed'
 $r.passed=$true
} catch { $r.error=$_.Exception.Message; Write-Output $_ } finally {
 if ($server -and -not $server.HasExited) { Stop-Process -Id $server.Id }
 $r | ConvertTo-Json -Depth 10 | Set-Content "$Root/qualification.json" -Encoding UTF8
 Stop-Transcript | Out-Null
}
if (-not $r.passed) { exit 1 }
