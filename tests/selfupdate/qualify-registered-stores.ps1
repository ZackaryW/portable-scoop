#Requires -Version 5.1
param(
    [Parameter(Mandatory=$true)][string]$Root,
    [string]$ExpectedVersion='0.3.2'
)
$ErrorActionPreference='Stop'
if ((Get-CimInstance Win32_ComputerSystem).Manufacturer -notmatch 'VMware') { throw 'Disposable VMware guest required' }
$Root=[IO.Path]::GetFullPath($Root)
$hostRoot=Join-Path $Root 'host'; $storeRoot=Join-Path $Root 'old-store'
if ((Test-Path -LiteralPath $hostRoot) -or (Test-Path -LiteralPath $storeRoot)) { throw 'Fresh fixture roots required' }
$oldLocal=$env:LOCALAPPDATA
$server=$null
$result=[ordered]@{passed=$false;error=$null;before=$null;after=$null;backend=$null}
function Assert($condition,[string]$message) { if (-not $condition) { throw $message } }
function Hash([string]$path) { if (Test-Path -LiteralPath $path) { return (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash }; return 'absent' }
function Tree([string]$path) {
    if (-not (Test-Path -LiteralPath $path)) { return 'absent' }
    return (@(Get-ChildItem -LiteralPath $path -File -Recurse -Force | Sort-Object FullName | ForEach-Object { $_.FullName.Substring($path.Length)+':'+(Hash $_.FullName) }) -join "`n")
}
function PSP([string[]]$arguments,[string]$log,[int]$expected=0) {
    $saved=$ErrorActionPreference; $ErrorActionPreference='Continue'
    try { $output=& "$hostRoot/psp.exe" @arguments 2>&1; $code=$LASTEXITCODE } finally { $ErrorActionPreference=$saved }
    $text=$output | Out-String
    $text | Set-Content -LiteralPath (Join-Path $Root ($log+'.log'))
    Assert ($code -eq $expected) "$log returned $code, expected ${expected}: $text"
    return $text
}
try {
    $originalRegistry=Hash (Join-Path $oldLocal 'portscoop/registry.json')
    $scoopBefore=Tree (Join-Path $env:USERPROFILE 'scoop')
    $userPath=[Environment]::GetEnvironmentVariable('PATH','User')
    $machinePath=[Environment]::GetEnvironmentVariable('PATH','Machine')
    # Only this process and its children use this registry. No persistent setting changes.
    $env:LOCALAPPDATA=Join-Path $Root 'profile'
    Expand-Archive -LiteralPath (Join-Path $Root 'host.zip') -DestinationPath $hostRoot
    Expand-Archive -LiteralPath (Join-Path $Root 'old.zip') -DestinationPath $storeRoot
    & "$storeRoot/setup.ps1" -Mode Portable -Destination $storeRoot -Name 'update-fixture'
    Assert ($LASTEXITCODE -eq 0) 'Old Store initialization failed'
    $result.before=(& "$storeRoot/psp.exe" --version | Out-String).Trim()
    Assert ($result.before -match '0\.3\.0') 'Expected an old 0.3.0 runtime'
    $payload=Join-Path $Root 'payload'; $http=Join-Path $Root 'http'
    New-Item -ItemType Directory -Path $payload,$http -Force | Out-Null
    [IO.File]::WriteAllText((Join-Path $payload 'fixture.cmd'),"@echo preserved portable data`r`n")
    [IO.File]::WriteAllText((Join-Path $payload 'data.txt'),'persisted fixture data')
    Compress-Archive -Path "$payload/*" -DestinationPath "$http/fixture.zip"
    $ready=Join-Path $Root 'server.json'
    $server=Start-Process -FilePath "$Root/server.exe" -ArgumentList @('-root',('"'+$http+'"'),'-ready-file',('"'+$ready+'"')) -PassThru -WindowStyle Hidden
    for ($i=0;$i -lt 50 -and -not (Test-Path -LiteralPath $ready);$i++) { Start-Sleep -Milliseconds 100 }
    Assert (Test-Path -LiteralPath $ready) 'Fixture server did not start'
    $url=(Get-Content -LiteralPath $ready -Raw|ConvertFrom-Json).url
    $bucket=Join-Path $storeRoot 'scoop/buckets/fixture/bucket'
    New-Item -ItemType Directory -Path $bucket -Force | Out-Null
    $manifest=@{version='1.0';description='Portable update fixture';homepage='https://example.com';license='MIT';url=($url+'/fixture.zip');hash=(Hash "$http/fixture.zip").ToLowerInvariant();bin='fixture.cmd';persist='data.txt'}
    [IO.File]::WriteAllText((Join-Path $bucket 'fixture.json'),($manifest|ConvertTo-Json),(New-Object Text.UTF8Encoding($false)))
    & "$storeRoot/psp.exe" install fixture/fixture
    Assert ($LASTEXITCODE -eq 0) 'Fixture application installation failed'
    $null=PSP @('env','register',$storeRoot,'--name','default') 'register'
    $null=PSP @('env','register',$storeRoot,'--name','alias') 'alias'
    $null=PSP @('env','default','default') 'default'
    $registryPath=Join-Path $env:LOCALAPPDATA 'portscoop/registry.json'
    $registry=Get-Content -LiteralPath $registryPath -Raw | ConvertFrom-Json
    $registry.entries | Add-Member -NotePropertyName offline -NotePropertyValue ([pscustomobject]@{id=[guid]::NewGuid().ToString();path="$Root/missing";relative_path=($Root.Substring(3)+'\missing')})
    $registry.entries | Add-Member -NotePropertyName stale -NotePropertyValue ([pscustomobject]@{id=[guid]::NewGuid().ToString();path=$storeRoot;relative_path=$storeRoot.Substring(3)})
    [IO.File]::WriteAllText($registryPath,($registry|ConvertTo-Json -Depth 8),(New-Object Text.UTF8Encoding($false)))
    $registryBefore=Hash $registryPath
    $preserved=@('.portscoop/environment.json','.portscoop/state/prepared.json','scoop/config.json')
    foreach ($relative in @('scoop/persist/fixture/keep.txt','scoop/buckets/fixture/keep.txt','scoop/cache/keep.txt')) {
        $path=Join-Path $storeRoot $relative
        New-Item -ItemType Directory -Path (Split-Path -Parent $path) -Force | Out-Null
        [IO.File]::WriteAllText($path,'preserved portable data')
        $preserved+=$relative
    }
    $preserved+=@('scoop/apps/fixture/1.0/fixture.cmd','scoop/apps/fixture/1.0/manifest.json','scoop/apps/fixture/1.0/install.json','scoop/shims/fixture.cmd','scoop/persist/fixture/data.txt')
    $hashes=@{}; foreach ($p in $preserved) { $hashes[$p]=Hash (Join-Path $storeRoot $p) }
    $appBefore=Tree "$storeRoot/scoop/apps/fixture"
    $shimsBefore=Tree "$storeRoot/scoop/shims"
    $persistBefore=Tree "$storeRoot/scoop/persist"
    $receiptBefore=Hash (Join-Path $storeRoot '.portscoop/state/runtime.json')
    $check=PSP @('self-update','--check') 'check' 2
    Assert ($check -match 'alias,default' -and $check -match 'offline' -and $check -match 'stale') 'Missing per-target check outcomes'
    Assert ((Hash (Join-Path $storeRoot '.portscoop/state/runtime.json')) -eq $receiptBefore) 'Check changed runtime receipt'
    $updated=PSP @('self-update') 'update' 2
    Assert (([regex]::Matches($updated,'Store runtime and managed backend updated')).Count -eq 1) 'Store not updated exactly once'
    Assert ($updated -match 'already current') 'Current Host did not remain current'
    $result.after=(& "$storeRoot/psp.exe" --version | Out-String).Trim()
    Assert ($result.after -match [regex]::Escape($ExpectedVersion)) 'Store version did not advance to expected release'
    $result.backend=(Get-Content "$storeRoot/.portscoop/backend/backend.json" -Raw|ConvertFrom-Json).patch_revision
    Assert ($result.backend -eq 'portable-operations-v1.0') 'Managed backend did not advance'
    foreach ($p in $preserved) { Assert ((Hash (Join-Path $storeRoot $p)) -eq $hashes[$p]) "Changed preserved data: $p" }
    Assert ((Tree "$storeRoot/scoop/apps/fixture") -eq $appBefore) 'Installed app changed'
    Assert ((Tree "$storeRoot/scoop/shims") -eq $shimsBefore) 'Installed shims changed'
    Assert ((Tree "$storeRoot/scoop/persist") -eq $persistBefore) 'Persisted app data changed'
    Assert ((Hash $registryPath) -eq $registryBefore) 'Registry changed during update'
    $status=PSP @('self-update','--status') 'status' 2
    Assert ($status -match 'completed: Updated to') 'Completed Store status absent'
    $again=PSP @('self-update') 'repeat' 2
    Assert ($again -notmatch 'managed backend updated') 'Repeated update replaced current Store'
    $backendOutput=& "$storeRoot/psp.exe" list 2>&1
    $backendOutput | Out-String | Set-Content "$Root/backend-list.log"
    Assert ($LASTEXITCODE -eq 0) 'Updated Store cannot dispatch its backend'
    $shimOutput=& "$storeRoot/scoop/shims/fixture.cmd" | Out-String
    Assert ($shimOutput -match 'preserved portable data') 'Preserved shim did not execute'
    Assert ((Hash (Join-Path $oldLocal 'portscoop/registry.json')) -eq $originalRegistry) 'Normal registry changed'
    Assert ((Tree (Join-Path $env:USERPROFILE 'scoop')) -eq $scoopBefore) 'Ordinary Scoop changed'
    Assert ([Environment]::GetEnvironmentVariable('PATH','User') -ceq $userPath) 'User PATH changed'
    Assert ([Environment]::GetEnvironmentVariable('PATH','Machine') -ceq $machinePath) 'Machine PATH changed'
    $result.passed=$true
} catch { $result.error=$_.Exception.Message; throw } finally {
    if ($null -ne $server -and -not $server.HasExited) { Stop-Process -Id $server.Id -ErrorAction SilentlyContinue }
    $env:LOCALAPPDATA=$oldLocal
    $result | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $Root 'result.json')
}
