# Disposable VMware guest only. Use a locally built release; nothing is published.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Bundle,[Parameter(Mandatory=$true)][string]$Results)
$ErrorActionPreference='Stop'
$root=Join-Path 'C:\Users\Public' ('PortscoopFirstStore-'+[guid]::NewGuid().ToString('N'))
$originalPath=$env:PATH;$originalLocal=$env:LOCALAPPDATA
$originalUserPath=[Environment]::GetEnvironmentVariable('PATH','User')
$originalProtocol=[Net.ServicePointManager]::SecurityProtocol
$status=[ordered]@{status='running';root=$root;powershell=$PSVersionTable.PSVersion.ToString();go_absent=(-not[bool](Get-Command go -ErrorAction SilentlyContinue));checks=@();error=$null}
function Assert([bool]$Condition,[string]$Message){if(-not$Condition){throw $Message}}
function Assert-EmptyStore([string]$Executable,[string[]]$Arguments){
    $output=& $Executable @Arguments|Out-String
    Assert ($LASTEXITCODE-eq 1-and$output-match"There aren't any apps installed") ('Unexpected empty Store response: '+$output)
}
try {
    Expand-Archive -LiteralPath $Bundle -DestinationPath $root
    Start-Transcript -Path (Join-Path $root 'first-store.log')|Out-Null
    $sentinel=Join-Path $root 'host-state.ps1'
    $before=& $sentinel
    $profile=Join-Path $root 'host-profile'
    $env:LOCALAPPDATA=$profile
    New-Item -ItemType Directory -Path $profile|Out-Null
    $global:PortscoopFirstStoreFixture=@{root=$root;answers=(New-Object 'Collections.Generic.Queue[string]')}
    function global:Invoke-RestMethod {
        param([string]$Uri,[int]$TimeoutSec)
        if($Uri-ne'https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/scripts/install.ps1'){throw 'Unexpected worker URL'}
        [IO.File]::ReadAllText((Join-Path $global:PortscoopFirstStoreFixture.root 'scripts\install.ps1'))
    }
    function global:Invoke-WebRequest {
        param([string]$Uri,[string]$OutFile,[switch]$UseBasicParsing,[int]$TimeoutSec)
        if($Uri-notmatch'^https://github\.com/ZackaryW/portable-scoop/releases/latest/download/(portscoop-windows-amd64\.zip(?:\.sha256)?)$'){throw 'Unexpected release URL'}
        Copy-Item -LiteralPath (Join-Path $global:PortscoopFirstStoreFixture.root $Matches[1]) -Destination $OutFile
    }
    function global:Read-Host {
        param([string]$Prompt)
        if(-not$global:PortscoopFirstStoreFixture.answers.Count){throw ('Unexpected prompt: '+$Prompt)}
        $global:PortscoopFirstStoreFixture.answers.Dequeue()
    }
    $entry=Join-Path $root 'get.ps1'
    $hostRoot=Join-Path $root 'Host with default'
    New-Item -ItemType Directory -Path $hostRoot|Out-Null
    foreach($answer in @('1','','n','yes')){$global:PortscoopFirstStoreFixture.answers.Enqueue($answer)}
    Push-Location -LiteralPath $hostRoot
    try{& {Invoke-Expression ([IO.File]::ReadAllText($entry))}}finally{Pop-Location}
    Assert ($global:PortscoopFirstStoreFixture.answers.Count-eq 0) 'Host wizard did not consume expected answers'
    $exe=Join-Path $hostRoot 'psp.exe'
    $store=Join-Path $hostRoot 'store'
    $identity=Get-Content -LiteralPath (Join-Path $store '.portscoop\environment.json') -Raw|ConvertFrom-Json
    $registryPath=Join-Path $profile 'portscoop\registry.json'
    $registry=Get-Content -LiteralPath $registryPath -Raw|ConvertFrom-Json
    Assert ($registry.default_id-eq$identity.id-and$registry.entries.default.id-eq$identity.id-and$registry.entries.default.path-ieq$store) 'First Store was not registered and selected'
    foreach($deployment in @($hostRoot,$store)){Assert (Test-Path -LiteralPath (Join-Path $deployment 'pspe.exe')) 'Host/default Store is missing execution launcher'}
    Assert-EmptyStore $exe @('list')
    Assert-EmptyStore (Join-Path $store 'psp.exe') @('list')
    Assert ($env:PATH-ceq$originalPath) 'Host No PATH choice was ignored'
    $status.checks+='Host wizard Yes creates the store subfolder, registers/selects default, and supports central and direct commands without PATH changes'

    $registryBefore=[IO.File]::ReadAllText($registryPath)
    & $exe env bootstrap
    Assert ($LASTEXITCODE-eq 0-and[IO.File]::ReadAllText($registryPath)-ceq$registryBefore) 'Repeated bootstrap changed the default'
    [IO.File]::WriteAllText((Join-Path $store 'user-data.txt'),'preserve')
    $second=Join-Path $root 'Second Store'
    & $exe env init $second --name Second
    Assert ($LASTEXITCODE-eq 0) 'Nested default Store blocked future initialization'
    Assert (Test-Path -LiteralPath (Join-Path $second 'pspe.exe')) 'Subsequent Store is missing execution launcher'
    Assert ([IO.File]::ReadAllText((Join-Path $store 'user-data.txt'))-ceq'preserve') 'Future initialization changed first Store data'
    Assert (-not(Test-Path -LiteralPath (Join-Path $second 'store'))) 'Nested Store was copied into another environment'
    $status.checks+='Repeated bootstrap preserves selection; mutable default Store does not block creation of additional independent Stores'

    $otherHost=Join-Path $root 'Another Host'
    & $entry -Mode Host -Destination $otherHost -CreateDefaultStore -NonInteractive
    Assert (-not(Test-Path -LiteralPath (Join-Path $otherHost 'store'))) 'Existing default caused another Store to be created'
    Assert ([IO.File]::ReadAllText($registryPath)-ceq$registryBefore) 'Another Host replaced the existing default'
    $status.checks+='Another Host with bootstrap requested preserves the existing default and creates no extra Store'

    $env:LOCALAPPDATA=Join-Path $root 'declined-profile'
    New-Item -ItemType Directory -Path $env:LOCALAPPDATA|Out-Null
    $declined=Join-Path $root 'Host only'
    foreach($answer in @('1',$declined,'n','no')){$global:PortscoopFirstStoreFixture.answers.Enqueue($answer)}
    & $entry
    Assert (-not(Test-Path -LiteralPath (Join-Path $declined 'store'))-and-not(Test-Path -LiteralPath (Join-Path $env:LOCALAPPDATA 'portscoop\registry.json'))) 'No created a Store or registration'
    Assert ($global:PortscoopFirstStoreFixture.answers.Count-eq 0) 'Opt-out wizard left unexpected answers'
    $status.checks+='Host wizard No leaves a Host-only deployment without a Store or registration'
    Assert ([Net.ServicePointManager]::SecurityProtocol-eq$originalProtocol) 'TLS settings leaked'
    $status.status='passed'
}catch{$status.status='failed';$status.error=$_.Exception.ToString()}
finally{
    [Environment]::SetEnvironmentVariable('PATH',$originalUserPath,'User')
    $env:PATH=$originalPath;$env:LOCALAPPDATA=$originalLocal
    [Net.ServicePointManager]::SecurityProtocol=$originalProtocol
    Remove-Item Function:\Invoke-RestMethod,Function:\Invoke-WebRequest,Function:\Read-Host -ErrorAction SilentlyContinue
    Remove-Variable PortscoopFirstStoreFixture -Scope Global -ErrorAction SilentlyContinue
    try{
        $after=& (Join-Path $root 'host-state.ps1')
        Assert (($before|ConvertTo-Json -Depth 6 -Compress)-ceq($after|ConvertTo-Json -Depth 6 -Compress)) 'Scoop sentinel or persistent environment changed'
        $status.checks+='Scoop sentinel and user/machine environment unchanged'
    }catch{$status.status='failed';$status.error=$_.Exception.ToString()}
    Stop-Transcript -ErrorAction SilentlyContinue|Out-Null
    $status|ConvertTo-Json -Depth 6|Set-Content -LiteralPath $Results -Encoding UTF8
}
if($status.status-ne'passed'){throw $status.error}
$status|ConvertTo-Json -Depth 6
