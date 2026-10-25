# Run with Windows PowerShell 5.1. Network calls are replaced inside this test
# process; the production installer contains no mock URLs or test-mode switches.
[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
$project=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$worker=Join-Path $project 'scripts\install.ps1'
$entry=Join-Path $project 'get.ps1'
$root=Join-Path ([IO.Path]::GetTempPath()) ('portscoop-installer-tests-'+[guid]::NewGuid().ToString('N'))
$payload=Join-Path $root 'payload'
New-Item -ItemType Directory -Path "$payload\backend" -Force|Out-Null
$stub=@'
[CmdletBinding()]
param([string]$Mode,[string]$Destination,[string]$Name,[switch]$AddToPath)
$ErrorActionPreference='Stop'
New-Item -ItemType Directory -Path $Destination -Force|Out-Null
$receipt=@{mode=$Mode;destination=$Destination;name=$Name;add_to_path=[bool]$AddToPath}
[IO.File]::WriteAllText($env:PORTSCOOP_TEST_RECEIPT,($receipt|ConvertTo-Json),(New-Object Text.UTF8Encoding($false)))
'@
[IO.File]::WriteAllText((Join-Path $payload 'setup.ps1'),$stub)
foreach($file in @('portscoop.exe','psp.exe','release.json','backend\backend.json')){[IO.File]::WriteAllText((Join-Path $payload $file),'fixture')}
$archive=Join-Path $root 'fixture.zip'
Compress-Archive -Path "$payload\*" -DestinationPath $archive
$global:PortscoopInstallerTest=@{archive=$archive;checksum=$null;calls=(New-Object 'Collections.Generic.List[string]');fail=$false;answers=(New-Object 'Collections.Generic.Queue[string]')}
function global:Invoke-RestMethod {
    param([string]$Uri,[int]$TimeoutSec)
    $global:PortscoopInstallerTest.calls.Add($Uri)
    if($Uri-notmatch'^https://raw\.githubusercontent\.com/ZackaryW/portable-scoop/(main|v[0-9A-Za-z._+-]+)/scripts/install\.ps1$'){throw "Unexpected script URL: $Uri"}
    return [IO.File]::ReadAllText($worker)
}
function global:Invoke-WebRequest {
    param([string]$Uri,[string]$OutFile,[switch]$UseBasicParsing,[int]$TimeoutSec)
    $global:PortscoopInstallerTest.calls.Add($Uri)
    if($global:PortscoopInstallerTest.fail){throw 'Fixture HTTP 404'}
    if($Uri-notmatch'^https://github\.com/ZackaryW/portable-scoop/releases/(latest/download|download/v[0-9A-Za-z._+-]+)/portscoop-windows-amd64\.zip(\.sha256)?$'){throw "Unexpected asset URL: $Uri"}
    if($Uri.EndsWith('.sha256')){
        $hash=$global:PortscoopInstallerTest.checksum
        if(-not$hash){$hash=(Get-FileHash -LiteralPath $global:PortscoopInstallerTest.archive).Hash}
        [IO.File]::WriteAllText($OutFile,($hash+'  portscoop-windows-amd64.zip'))
    }else{Copy-Item -LiteralPath $global:PortscoopInstallerTest.archive -Destination $OutFile}
}
function global:Read-Host {
    param([string]$Prompt)
    if(-not$global:PortscoopInstallerTest.answers.Count){throw "Unexpected prompt: $Prompt"}
    return $global:PortscoopInstallerTest.answers.Dequeue()
}
function Assert([bool]$Condition,[string]$Message){if(-not$Condition){throw $Message}}
function Expect-Failure([scriptblock]$Action,[string]$Pattern){
    $caught=$false
    try{& $Action}catch{$caught=$true;Assert ($_.Exception.Message-match$Pattern) ('Wrong failure: '+$_.Exception.Message)}
    Assert $caught 'Expected installation to fail'
}
$originalPath=$env:PATH
$originalReceipt=$env:PORTSCOOP_TEST_RECEIPT
$originalProtocol=[Net.ServicePointManager]::SecurityProtocol
$passed=New-Object 'Collections.Generic.List[string]'
try {
    $env:PORTSCOOP_TEST_RECEIPT=Join-Path $root 'receipt.json'
    $destination=Join-Path $root 'store (work) & data'
    $displayName='Work '+[char]0x03c0+' "quoted"'
    $options=@{Mode='Store';Destination=$destination;Name=$displayName;NonInteractive=$true}
    & $entry @options
    $receipt=Get-Content -LiteralPath $env:PORTSCOOP_TEST_RECEIPT -Raw -Encoding UTF8|ConvertFrom-Json
    Assert ($receipt.mode-eq'Portable'-and$receipt.destination-ceq$destination-and$receipt.name-ceq$displayName-and-not$receipt.add_to_path) 'Store arguments changed'
    Assert ($env:PATH-ceq$originalPath) 'Store changed caller PATH'
    Assert ([Net.ServicePointManager]::SecurityProtocol-eq$originalProtocol) 'TLS settings leaked to caller'
    $passed.Add('store options, Unicode/quotes, caller PATH and TLS preservation')

    $destination=Join-Path $root 'host launcher'
    & $entry -Mode Host -Destination $destination -AddToPath -Version '0.2.0' -NonInteractive
    $receipt=Get-Content -LiteralPath $env:PORTSCOOP_TEST_RECEIPT -Raw|ConvertFrom-Json
    Assert ($receipt.mode-eq'Central'-and$receipt.add_to_path) 'Host options were not forwarded'
    Assert ($env:PATH.StartsWith($destination+';')) 'Host launcher was not activated in current console'
    Assert (@($global:PortscoopInstallerTest.calls|Where-Object {$_-like'*/v0.2.0/scripts/install.ps1'}).Count-eq 1) 'Pinned installer tag was not selected'
    Assert (@($global:PortscoopInstallerTest.calls|Where-Object {$_-like'*/download/v0.2.0/*'}).Count-eq 2) 'Pinned release assets were not selected'
    $env:PATH=$originalPath
    $passed.Add('host activation and version-pinned script/assets')

    $destination=Join-Path $root 'wizard store'
    foreach($answer in @('2',$destination,'Wizard Store')){$global:PortscoopInstallerTest.answers.Enqueue($answer)}
    $code=[IO.File]::ReadAllText($entry)
    Invoke-Expression $code
    $receipt=Get-Content -LiteralPath $env:PORTSCOOP_TEST_RECEIPT -Raw|ConvertFrom-Json
    Assert ($receipt.mode-eq'Portable'-and$receipt.name-eq'Wizard Store') 'IEX wizard did not choose Store'
    Assert ($global:PortscoopInstallerTest.answers.Count-eq 0) 'Wizard did not consume expected choices'
    $passed.Add('Invoke-Expression entry and interactive Store selection')

    $destination=Join-Path $root 'existing'
    New-Item -ItemType Directory -Path $destination|Out-Null
    [IO.File]::WriteAllText((Join-Path $destination 'sentinel'),'preserve')
    $count=$global:PortscoopInstallerTest.calls.Count
    Expect-Failure {& $worker -Mode Store -Destination $destination -NonInteractive} 'empty'
    Assert ($global:PortscoopInstallerTest.calls.Count-eq$count) 'Existing destination triggered downloads'
    Assert ([IO.File]::ReadAllText((Join-Path $destination 'sentinel'))-ceq'preserve') 'Existing data changed'
    $passed.Add('non-empty destination rejected before downloads')

    $destination=Join-Path $root 'bad checksum'
    $global:PortscoopInstallerTest.checksum='0'*64
    Expect-Failure {& $worker -Mode Store -Destination $destination -NonInteractive} 'SHA-256 mismatch'
    Assert (-not(Test-Path -LiteralPath $destination)) 'Checksum failure published a destination'
    $global:PortscoopInstallerTest.checksum=$null
    $passed.Add('checksum mismatch blocks setup')

    $global:PortscoopInstallerTest.fail=$true
    Expect-Failure {& $worker -Mode Store -Destination (Join-Path $root 'missing release') -NonInteractive} 'published Portscoop release'
    $global:PortscoopInstallerTest.fail=$false
    $passed.Add('missing release has actionable failure')

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $unsafe=Join-Path $root 'unsafe.zip'
    Copy-Item -LiteralPath $archive -Destination $unsafe
    $zip=[IO.Compression.ZipFile]::Open($unsafe,[IO.Compression.ZipArchiveMode]::Update)
    try{$entryInZip=$zip.CreateEntry('../outside.txt');$writer=New-Object IO.StreamWriter($entryInZip.Open());try{$writer.Write('unsafe')}finally{$writer.Dispose()}}finally{$zip.Dispose()}
    $global:PortscoopInstallerTest.archive=$unsafe
    Expect-Failure {& $worker -Mode Store -Destination (Join-Path $root 'unsafe store') -NonInteractive} 'unsafe extraction path'
    Assert (-not(Test-Path -LiteralPath (Join-Path $root 'outside.txt'))) 'ZIP traversal escaped extraction'
    $global:PortscoopInstallerTest.archive=$archive
    $passed.Add('ZIP traversal rejected before extraction')

    Expect-Failure {& $worker -Mode Store -Destination (Join-Path $root 'invalid options') -AddToPath -NonInteractive} 'does not add'
    Expect-Failure {& $worker -Mode Host -Destination (Join-Path $root 'invalid options') -RegisterAs work -NonInteractive} 'only to Store'
    Expect-Failure {& $worker -NonInteractive} 'requires Mode'
    Expect-Failure {& $entry -Mode Host -Destination (Join-Path $root 'invalid tag') -Version '../main'} 'Invalid release'
    $passed.Add('mode-specific options and unsafe versions rejected')
    [pscustomobject]@{status='passed';checks=@($passed)}|ConvertTo-Json
} finally {
    $env:PATH=$originalPath;$env:PORTSCOOP_TEST_RECEIPT=$originalReceipt
    [Net.ServicePointManager]::SecurityProtocol=$originalProtocol
    Remove-Item Function:\Invoke-RestMethod,Function:\Invoke-WebRequest,Function:\Read-Host -ErrorAction SilentlyContinue
    Remove-Variable PortscoopInstallerTest -Scope Global -ErrorAction SilentlyContinue
    $target=[IO.Path]::GetFullPath($root)
    $parent=[IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\','/')+'\'
    if($target.StartsWith($parent,[StringComparison]::OrdinalIgnoreCase)-and[IO.Path]::GetFileName($target).StartsWith('portscoop-installer-tests-')){Remove-Item -LiteralPath $target -Recurse -Force}
}
