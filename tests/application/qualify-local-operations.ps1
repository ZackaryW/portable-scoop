#Requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$Root)
$ErrorActionPreference='Stop'
if ((Get-CimInstance Win32_ComputerSystem).Manufacturer -notmatch 'VMware') {throw 'VMware only'}
$Root=[IO.Path]::GetFullPath($Root);$store="$Root/store";$other="$Root/other"
if ((Test-Path $store)-or(Test-Path $other)){throw 'Fresh Stores required'}
Start-Transcript -Path "$Root/qualification.log" | Out-Null
$r=[ordered]@{passed=$false;kind='local-operation-integration';apps=@();error=$null};$server=$null
function Assert($b,$m){if(-not $b){throw $m}}
function Fingerprint($path){if(-not(Test-Path -LiteralPath $path)){return 'absent'};return (@(Get-ChildItem -LiteralPath $path -File -Recurse -Force | Sort-Object FullName | ForEach-Object{$_.FullName.Substring($path.Length)+':'+(Get-FileHash -LiteralPath $_.FullName).Hash}) -join "`n")}
function PSP([string[]]$arguments){& "$store/psp.exe" @arguments;if($LASTEXITCODE -ne 0){throw "psp failed: $arguments"}}
try {
 $scoopBefore=Fingerprint "$env:USERPROFILE/scoop"
 $userPath=[Environment]::GetEnvironmentVariable('PATH','User');$machinePath=[Environment]::GetEnvironmentVariable('PATH','Machine')
 foreach($s in @($store,$other)) {
  Expand-Archive -LiteralPath "$Root/release.zip" -DestinationPath $s
  & "$s/setup.ps1" -Mode Portable -Destination $s -Name ([IO.Path]::GetFileName($s))
  Assert ($LASTEXITCODE -eq 0) 'Setup failed'
 }
 $otherBefore=Fingerprint $other
 $bucket="$store/scoop/buckets/fixtures/bucket"
 New-Item -ItemType Directory -Path $bucket -Force | Out-Null
 foreach($name in @('rufus','nunit-console')) {
  Copy-Item -LiteralPath "$Root/$name.json" -Destination $bucket
  PSP @('install',"fixtures/$name")
  $m=Get-Content "$bucket/$name.json" -Raw|ConvertFrom-Json
  $physical="$store/scoop/apps/$name/$($m.version)"
  if($name -eq 'rufus') {
   $configuration="$store/scoop/persist/rufus/rufus.ini"
   Assert ([IO.File]::ReadAllText($configuration) -ceq "UpdateCheckInterval = -1`r`n") 'Rufus literal default differs'
   [IO.File]::WriteAllText($configuration,'preserve custom data')
   PSP @('reset','rufus')
   Assert ([IO.File]::ReadAllText($configuration) -ceq 'preserve custom data') 'Rufus reset lost persistence'
  } else {
   Assert ([IO.File]::ReadAllText("$physical/nunit.scoop.addins") -ceq '../../nunit-extension-*/current/     # find extensions installed under scoop') 'NUnit post-install literal differs'
   & "$store/pspe.exe" nunit3-console --help
   Assert ($LASTEXITCODE -eq 0) 'NUnit CLI launch failed'
  }
  $r.apps+=@{name=$name;version=$m.version;installed=$true;literal_verified=$true}
 }
 $payload="$Root/payload";$contents="$Root/contents"
 New-Item -ItemType Directory -Path $payload,$contents -Force | Out-Null
 [IO.File]::WriteAllText("$contents/default.ini",'packaged default')
 Compress-Archive -Path "$contents/*" -DestinationPath "$payload/mixed.zip"
 $ready="$Root/server-ready.json"
 $server=Start-Process -FilePath "$Root/probe-server.exe" -ArgumentList @('-root',('"'+$payload+'"'),'-ready-file',('"'+$ready+'"')) -PassThru -WindowStyle Hidden
 $deadline=[DateTime]::UtcNow.AddSeconds(10)
 while(-not(Test-Path $ready)){if($server.HasExited -or [DateTime]::UtcNow -gt $deadline){throw 'Fixture server failed'};Start-Sleep -Milliseconds 100}
 $url=(Get-Content $ready -Raw|ConvertFrom-Json).url
 $m=@{version='1';url="$url/mixed.zip";hash=(Get-FileHash "$payload/mixed.zip").Hash.ToLowerInvariant();persist='profile';pre_install=@('New-Item "$dir\cache" -ItemType Directory -Force | Out-Null','if (!(Test-Path "$persist_dir\profile\options.ini")) { Copy-Item "$dir\default.ini" "$dir\profile\options.ini" -Force }');post_install='New-Item "$dir\profile\after.ini" -ItemType File -Value ''post-install default'' | Out-Null'}
 $m|ConvertTo-Json -Depth 8|Set-Content "$bucket/nimbus.json" -Encoding UTF8
 PSP @('install','fixtures/nimbus')
 $persist="$store/scoop/persist/nimbus/profile"
 Assert ([IO.File]::ReadAllText("$persist/options.ini") -ceq 'packaged default') 'Packaged default missing'
 Assert ([IO.File]::ReadAllText("$persist/after.ini") -ceq 'post-install default') 'Post-install persistence mapping failed'
 Assert ([IO.Directory]::Exists("$store/scoop/apps/nimbus/1/cache")) 'Directory creation failed'
 [IO.File]::WriteAllText("$persist/options.ini",'custom saved default')
 PSP @('reset','nimbus')
 Assert ([IO.File]::ReadAllText("$persist/options.ini") -ceq 'custom saved default') 'Synthetic reset lost data'
 $r.synthetic=$true
 $r.otherStoreUnchanged=(Fingerprint $other) -ceq $otherBefore
 $r.ordinaryScoopUnchanged=(Fingerprint "$env:USERPROFILE/scoop") -ceq $scoopBefore
 $r.persistentPathUnchanged=$userPath -ceq [Environment]::GetEnvironmentVariable('PATH','User') -and $machinePath -ceq [Environment]::GetEnvironmentVariable('PATH','Machine')
 Assert ($r.otherStoreUnchanged -and $r.ordinaryScoopUnchanged -and $r.persistentPathUnchanged) 'Store or host isolation failed'
 $r.passed=$true
} catch {$r.error=$_.Exception.Message;Write-Output $_} finally {
 if($server -and -not $server.HasExited){Stop-Process -Id $server.Id}
 $r|ConvertTo-Json -Depth 10|Set-Content "$Root/qualification.json" -Encoding UTF8
 Stop-Transcript|Out-Null
}
if(-not $r.passed){exit 1}
