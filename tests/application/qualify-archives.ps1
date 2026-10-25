#Requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$Root)
$ErrorActionPreference='Stop'
if ((Get-CimInstance Win32_ComputerSystem).Manufacturer -notmatch 'VMware') { throw 'VMware only' }
$Root=[IO.Path]::GetFullPath($Root);$store="$Root/store"
if (Test-Path $store) { throw 'Fresh Store required' }
Start-Transcript -Path "$Root/qualification.log" | Out-Null
$r=[ordered]@{passed=$false;kind='managed-archive-integration';apps=@();error=$null}
function Assert($b,$m) { if(-not $b){throw $m} }
function Fingerprint($path) { if (-not(Test-Path -LiteralPath $path)){return 'absent'};return (@(Get-ChildItem -LiteralPath $path -File -Recurse -Force | Sort-Object FullName | ForEach-Object { $_.FullName.Substring($path.Length)+':'+(Get-FileHash -LiteralPath $_.FullName).Hash }) -join "`n") }
try {
 $scoopBefore=Fingerprint "$env:USERPROFILE/scoop"
 $userPath=[Environment]::GetEnvironmentVariable('PATH','User');$machinePath=[Environment]::GetEnvironmentVariable('PATH','Machine')
 Expand-Archive -LiteralPath "$Root/release.zip" -DestinationPath $store
 & "$store/setup.ps1" -Mode Portable -Destination $store -Name managed-archives
 Assert ($LASTEXITCODE -eq 0) 'Setup failed'
 $bucket="$store/scoop/buckets/main/bucket"
 New-Item -ItemType Directory -Path $bucket -Force | Out-Null
 foreach ($name in @('file','chroma','lxc')) {
  Copy-Item -LiteralPath "$Root/$name.json" -Destination $bucket
  & "$store/psp.exe" install "main/$name"
  Assert ($LASTEXITCODE -eq 0) "$name install failed"
  $manifest=Get-Content "$bucket/$name.json" -Raw | ConvertFrom-Json
  $exe="$store/scoop/apps/$name/$($manifest.version)/$name.exe"
  Assert (Test-Path -LiteralPath $exe -PathType Leaf) "$name executable missing"
  & "$store/pspe.exe" $name --version
  Assert ($LASTEXITCODE -eq 0) "$name version command failed"
  $r.apps+=@{name=$name;version=$manifest.version;installed=$true;cli_launched=$true;executable_sha256=(Get-FileHash -LiteralPath $exe).Hash.ToLowerInvariant()}
 }
 $r.ordinaryScoopUnchanged=(Fingerprint "$env:USERPROFILE/scoop") -ceq $scoopBefore
 $r.persistentPathUnchanged=$userPath -ceq [Environment]::GetEnvironmentVariable('PATH','User') -and $machinePath -ceq [Environment]::GetEnvironmentVariable('PATH','Machine')
 Assert ($r.ordinaryScoopUnchanged -and $r.persistentPathUnchanged) 'Guest host state changed'
 $r.passed=$true
} catch {$r.error=$_.Exception.Message;Write-Output $_} finally {
 $r | ConvertTo-Json -Depth 10 | Set-Content "$Root/qualification.json" -Encoding UTF8
 Stop-Transcript | Out-Null
}
if(-not $r.passed){exit 1}
