# Download a published Portscoop binary release, verify it, then reuse setup.ps1.
# This file runs in the child script scope created by get.ps1 or normal -File use.
[CmdletBinding()]
param(
    [ValidateSet('Host','Store')][string]$Mode,
    [string]$Destination,
    [string]$Name,
    [switch]$AddToPath,
    [switch]$NoPath,
    [switch]$CreateDefaultStore,
    [switch]$NoDefaultStore,
    [string]$Version,
    [string]$RegisterAs,
    [switch]$SetDefault,
    [switch]$NonInteractive
)
$ErrorActionPreference='Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT -or -not [Environment]::Is64BitProcess -or $env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { throw 'Portscoop requires Windows x64 and a 64-bit PowerShell session' }
if ($AddToPath -and $NoPath) { throw 'Choose AddToPath or NoPath, not both' }
if ($CreateDefaultStore -and $NoDefaultStore) { throw 'Choose CreateDefaultStore or NoDefaultStore, not both' }
if ($SetDefault -and -not $RegisterAs) { throw 'SetDefault requires RegisterAs' }
if ($RegisterAs -and $RegisterAs -notmatch '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$') { throw 'RegisterAs must be a valid host alias' }
if ($Version -and $Version -notmatch '^v?[0-9][0-9A-Za-z._+-]*$') { throw 'Invalid release version' }
$wizard=-not $Mode
if ($wizard) {
    if ($NonInteractive) { throw 'NonInteractive installation requires Mode' }
    Write-Host 'Portscoop setup' -ForegroundColor Cyan
    Write-Host '  1. Host  - central launcher that finds registered portable stores'
    Write-Host '  2. Store - independent portable packages and state'
    Write-Host 'Both modes default to installing in your current folder.'
    do { $answer=Read-Host 'Choose Host or Store [1/2]' } until ($answer -in @('1','2','Host','Store'))
    $Mode=if($answer -in @('1','Host')){'Host'}else{'Store'}
}
if ($Mode -eq 'Host' -and ($RegisterAs -or $SetDefault -or $Name)) { throw 'Name and registration options apply only to Store setup' }
if ($Mode -eq 'Store' -and $AddToPath) { throw 'Store setup does not add portable paths to the host PATH' }
if ($Mode -eq 'Store' -and ($CreateDefaultStore -or $NoDefaultStore)) { throw 'Default Store bootstrap options apply only to Host setup' }
if (-not $Destination) {
    $default=(Get-Location).ProviderPath
    if ($NonInteractive) { $Destination=$default } else {
        $answer=Read-Host "Destination folder (Enter = current folder) [$default]"
        $Destination=if([string]::IsNullOrWhiteSpace($answer)){$default}else{$answer}
    }
}
$Destination=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Destination)
$Destination=[IO.Path]::GetFullPath($Destination).TrimEnd('\','/')
if ($Destination.TrimEnd('\','/') -ieq [IO.Path]::GetPathRoot($Destination).TrimEnd('\','/')) { throw 'Choose a folder, not a drive root' }
if ($Mode -eq 'Store' -and -not $Name) {
    $Name=Split-Path -Leaf $Destination.TrimEnd('\','/')
    if($wizard){$answer=Read-Host "Store name [$Name]";if(-not[string]::IsNullOrWhiteSpace($answer)){$Name=$answer}}
}
if ($Mode -eq 'Host' -and $wizard -and -not $PSBoundParameters.ContainsKey('AddToPath') -and -not $NoPath) {
    do{$answer=Read-Host 'Add the host launcher to your user PATH? [Y/n]'}until($answer -in @('','y','yes','n','no'))
    $AddToPath=$answer -in @('','y','yes')
}
if ($Mode -eq 'Host' -and -not $NonInteractive -and -not $PSBoundParameters.ContainsKey('CreateDefaultStore') -and -not $NoDefaultStore) {
    $firstStore=Join-Path $Destination 'store'
    do{$answer=Read-Host "Create a first default Store in '$firstStore'? [Y/n]"}until($answer -in @('','y','yes','n','no'))
    $CreateDefaultStore=$answer -in @('','y','yes')
}
$powerShell=Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
if(-not(Test-Path -LiteralPath $powerShell -PathType Leaf)){throw 'Windows PowerShell 5.1 is required'}
$pathBytes=[Text.Encoding]::UTF8.GetBytes($Destination.ToUpperInvariant())
$sha=[Security.Cryptography.SHA256]::Create()
try{$lockName=([BitConverter]::ToString($sha.ComputeHash($pathBytes))).Replace('-','')}finally{$sha.Dispose()}
$mutex=New-Object Threading.Mutex($false,('Local\PortscoopSetup-'+$lockName))
$locked=$false;$temporary=$null
$previousProtocol=[Net.ServicePointManager]::SecurityProtocol
try {
    try{$locked=$mutex.WaitOne(0)}catch [Threading.AbandonedMutexException]{$locked=$true}
    if(-not$locked){throw 'Another installer is already using this destination'}
    if(Test-Path -LiteralPath $Destination){
        if(-not(Test-Path -LiteralPath $Destination -PathType Container)-or@(Get-ChildItem -LiteralPath $Destination -Force).Count){throw "Destination must be empty: $Destination. Create and enter an empty folder, or supply -Destination with an empty folder. Existing files were preserved."}
    }
    [Net.ServicePointManager]::SecurityProtocol=$previousProtocol -bor [Net.SecurityProtocolType]::Tls12
    $base='https://github.com/ZackaryW/portable-scoop/releases/latest/download'
    if($Version){$tag=if($Version.StartsWith('v')){$Version}else{'v'+$Version};$base='https://github.com/ZackaryW/portable-scoop/releases/download/'+$tag}
    $asset='portscoop-windows-amd64.zip'
    $temporary=Join-Path ([IO.Path]::GetTempPath()) ('portscoop-setup-'+[guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporary|Out-Null
    $archive=Join-Path $temporary $asset
    $checksum=Join-Path $temporary ($asset+'.sha256')
    Write-Host "Downloading Portscoop for $Mode setup..."
    try {
        Invoke-WebRequest -Uri ($base+'/'+$asset+'.sha256') -OutFile $checksum -UseBasicParsing -TimeoutSec 60
        Invoke-WebRequest -Uri ($base+'/'+$asset) -OutFile $archive -UseBasicParsing -TimeoutSec 300
    }catch{throw ('Could not download the published Portscoop release. Check the release/version and network connection. '+$_.Exception.Message)}
    $line=[IO.File]::ReadAllText($checksum).Trim()
    if($line -notmatch '^([a-fA-F0-9]{64})\s+\*?portscoop-windows-amd64\.zip$'){throw 'Invalid release SHA-256 file'}
    $expected=$Matches[1]
    if((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ine $expected){throw 'Release SHA-256 mismatch; no installation was performed'}
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip=[IO.Compression.ZipFile]::OpenRead($archive)
    $seen=New-Object 'Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    try {
        foreach($entry in $zip.Entries){
            $relative=$entry.FullName.Replace('\','/').TrimEnd('/')
            if(-not$relative-or$relative.StartsWith('/')-or$relative.Contains(':')-or@($relative.Split('/')|Where-Object {$_ -in @('.','..','')-or$_.EndsWith('.')-or$_.EndsWith(' ')}).Count){throw 'Release ZIP contains an unsafe extraction path'}
            if(-not$seen.Add($relative)){throw 'Release ZIP contains duplicate extraction paths'}
            if((($entry.ExternalAttributes -shr 16)-band 0xf000)-eq 0xa000){throw 'Release ZIP contains an unsupported symbolic link'}
        }
    }finally{$zip.Dispose()}
    $payload=Join-Path $temporary 'payload'
    Expand-Archive -LiteralPath $archive -DestinationPath $payload
    foreach($required in @('setup.ps1','release.json','portscoop.exe','psp.exe','pspe.exe','backend\backend.json')){
        if(-not(Test-Path -LiteralPath (Join-Path $payload $required) -PathType Leaf)){throw "Release payload is missing $required"}
    }
    $supportsBootstrap=(Get-Command (Join-Path $payload 'setup.ps1')).Parameters.ContainsKey('CreateDefaultStore')
    if($CreateDefaultStore-and-not$supportsBootstrap){
        throw 'This release does not support first default Store setup. Choose a newer release with that feature, or use -NoDefaultStore.'
    }
    # Keep folder/name values in JSON rather than interpolating PowerShell code
    # or relying on Windows PowerShell's native argument reconstruction.
    $settings=@{setup=(Join-Path $payload 'setup.ps1');mode=$(if($Mode-eq'Host'){'Central'}else{'Portable'});destination=$Destination;name=$Name;add_to_path=[bool]$AddToPath;create_default_store=[bool]$CreateDefaultStore}
    $settingsFile=Join-Path $temporary 'settings.json'
    [IO.File]::WriteAllText($settingsFile,($settings|ConvertTo-Json),(New-Object Text.UTF8Encoding($false)))
    $runner=Join-Path $temporary 'run-setup.ps1'
    $runnerSource=@'
param([string]$SettingsFile)
$ErrorActionPreference='Stop'
$settings=Get-Content -LiteralPath $SettingsFile -Raw -Encoding UTF8|ConvertFrom-Json
$parameters=@{Mode=$settings.mode;Destination=$settings.destination}
if($settings.name){$parameters.Name=$settings.name}
if($settings.add_to_path){$parameters.AddToPath=$true}
if($settings.create_default_store){$parameters.CreateDefaultStore=$true}
& $settings.setup @parameters
'@
    [IO.File]::WriteAllText($runner,$runnerSource,(New-Object Text.UTF8Encoding($false)))
    & $powerShell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $runner -SettingsFile $settingsFile
    if($LASTEXITCODE-ne 0){throw 'Setup did not complete; inspect the destination before retrying'}
    $exe=Join-Path $Destination 'psp.exe'
    if($Mode-eq'Store'-and$RegisterAs){
        & $exe env register $Destination --name $RegisterAs
        if($LASTEXITCODE-ne 0){throw 'The store was created, but host registration failed'}
        if($SetDefault){& $exe env default $RegisterAs;if($LASTEXITCODE-ne 0){throw 'The store was registered, but default selection failed'}}
    }
    if($Mode-eq'Host'-and$AddToPath){
        $parts=@($env:PATH-split';')
        if(-not@($parts|Where-Object {$_.TrimEnd('\')-ieq$Destination.TrimEnd('\')}).Count){$env:PATH=$Destination+';'+$env:PATH}
    }
    Write-Host "Portscoop $Mode is ready at $Destination" -ForegroundColor Green
    if($Mode-eq'Host'){
        if($CreateDefaultStore){Write-Host 'The first default Store is configured; any existing default selection was preserved.'}elseif($supportsBootstrap){Write-Host 'Later: run psp env bootstrap to create a first default Store in the store subfolder.'}else{Write-Host 'Next: create/register a store and choose its default with env init/register/default.'}
        Write-Host ('Run: & "'+$exe+'" env list')
    }else{
        Write-Host ('Run: & "'+$exe+'" list')
        if(-not$RegisterAs){Write-Host 'This store works directly without host registration.'}
    }
} finally {
    [Net.ServicePointManager]::SecurityProtocol=$previousProtocol
    if($temporary){
        $tempRoot=[IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\','/')+[IO.Path]::DirectorySeparatorChar
        $target=[IO.Path]::GetFullPath($temporary)
        if(-not$target.StartsWith($tempRoot,[StringComparison]::OrdinalIgnoreCase)-or-not([IO.Path]::GetFileName($target)).StartsWith('portscoop-setup-')){throw 'Temporary cleanup path validation failed'}
        Remove-Item -LiteralPath $target -Recurse -Force -ErrorAction SilentlyContinue
    }
    if($locked){$mutex.ReleaseMutex()};$mutex.Dispose()
}
