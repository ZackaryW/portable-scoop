# Run only in the disposable Windows guest. Downloads are redirected to the
# supplied real release; setup, payload verification and initialization are real.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Bundle,[Parameter(Mandatory=$true)][string]$Results)
$ErrorActionPreference='Stop'
$root=Join-Path 'C:\Users\Public' ('PortscoopInstaller-'+[guid]::NewGuid().ToString('N'))
$originalPath=$env:PATH
$originalLocal=$env:LOCALAPPDATA
$originalUserPath=[Environment]::GetEnvironmentVariable('PATH','User')
$originalProtocol=[Net.ServicePointManager]::SecurityProtocol
$status=[ordered]@{status='running';root=$root;checks=@();error=$null}
function Assert([bool]$Condition,[string]$Message){if(-not$Condition){throw $Message}}
function Assert-EmptyStore([string]$Executable,[string[]]$Arguments){
    $output=& $Executable @Arguments|Out-String
    # Pinned Scoop intentionally exits 1 when list finds no installed apps.
    Assert ($LASTEXITCODE-eq 1-and$output-match"There aren't any apps installed") ('Expected empty Store response: '+$output)
}
try {
    Expand-Archive -LiteralPath $Bundle -DestinationPath $root
    Start-Transcript -Path (Join-Path $root 'qualification.log')|Out-Null
    $before=& (Join-Path $root 'host-state.ps1')
    $before|ConvertTo-Json -Depth 6|Set-Content -LiteralPath (Join-Path $root 'host-before.json') -Encoding UTF8
    $env:LOCALAPPDATA=Join-Path $root 'host-profile'
    New-Item -ItemType Directory -Path $env:LOCALAPPDATA|Out-Null
    $global:PortscoopReleaseFixture=$root
    function global:Invoke-RestMethod {
        param([string]$Uri,[int]$TimeoutSec)
        if($Uri-ne'https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/scripts/install.ps1'){throw 'Unexpected installer URL'}
        [IO.File]::ReadAllText((Join-Path $global:PortscoopReleaseFixture 'scripts\install.ps1'))
    }
    function global:Invoke-WebRequest {
        param([string]$Uri,[string]$OutFile,[switch]$UseBasicParsing,[int]$TimeoutSec)
        if($Uri-notmatch'^https://github\.com/ZackaryW/portable-scoop/releases/latest/download/(portscoop-windows-amd64\.zip(?:\.sha256)?)$'){throw 'Unexpected release URL'}
        Copy-Item -LiteralPath (Join-Path $global:PortscoopReleaseFixture $Matches[1]) -Destination $OutFile
    }
    $entry=Join-Path $root 'get.ps1'
    $hostRoot=Join-Path $root 'Host Launcher'
    & $entry -Mode Host -Destination $hostRoot -AddToPath -NonInteractive
    Assert ($env:PATH.StartsWith($hostRoot+';')) 'Host was not activated in current console'
    $expectedUserPath=$originalUserPath
    if($expectedUserPath-and-not$expectedUserPath.EndsWith(';')){$expectedUserPath+=';'}
    $expectedUserPath+=$hostRoot
    Assert ([Environment]::GetEnvironmentVariable('PATH','User')-ceq$expectedUserPath) 'Host modified more than its requested user PATH entry'
    $listing=psp env list|Out-String
    Assert ($LASTEXITCODE-eq 0-and$listing-match'ALIAS') 'Bare psp env list failed after Host setup'
    pspe --version | Out-Null
    Assert ($LASTEXITCODE-eq 0) 'Bare pspe version failed after Host PATH setup'
    $status.checks+='Host setup, exact user PATH addition, and immediate bare psp env list'

    $work=Join-Path $root 'Stores\Work (portable)'
    $personal=Join-Path $root 'Stores\Personal & tools'
    $pathAfterHost=$env:PATH
    & $entry -Mode Store -Destination $work -Name Work -NonInteractive
    Assert (-not(Test-Path -LiteralPath (Join-Path $env:LOCALAPPDATA 'portscoop\registry.json'))) 'Store registered implicitly'
    & $entry -Mode Store -Destination $personal -Name Personal -RegisterAs personal -SetDefault -NonInteractive
    Assert ($env:PATH-ceq$pathAfterHost) 'Store modified caller PATH'
    foreach($deployment in @($hostRoot,$work,$personal)){
        foreach($name in @('portscoop.exe','psp.exe','pspe.exe')){Assert (Test-Path -LiteralPath (Join-Path $deployment $name)) ('Missing deployment launcher: '+$name)}
        & (Join-Path $deployment 'pspe.exe') --help | Out-Null
        Assert ($LASTEXITCODE-eq 0) 'Execution help failed in deployment'
    }
    $workIdentity=Get-Content -LiteralPath (Join-Path $work '.portscoop\environment.json') -Raw|ConvertFrom-Json
    $personalIdentity=Get-Content -LiteralPath (Join-Path $personal '.portscoop\environment.json') -Raw|ConvertFrom-Json
    Assert ($workIdentity.id-ne$personalIdentity.id) 'Independent stores share an identity'
    $status.checks+='Two independent initialized stores, explicit registration/default, and Store PATH preservation'

    Assert-EmptyStore (Join-Path $work 'psp.exe') @('list')
    Assert-EmptyStore (Join-Path $hostRoot 'psp.exe') @('list')
    psp env register $work --name work
    Assert ($LASTEXITCODE-eq 0) 'Registering work failed'
    Assert-EmptyStore (Join-Path $hostRoot 'psp.exe') @('--env','work','list')
    $listing=psp env list|Out-String
    Assert ($LASTEXITCODE-eq 0-and$listing-match'personal'-and$listing-match'work') 'Registry listing incomplete'
    [IO.File]::WriteAllText((Join-Path $root 'env-list.txt'),$listing)
    $status.checks+='Direct Store execution, central default delegation, explicit selection, and both aliases listed'

    $sentinel=Join-Path $work 'keep.txt'
    [IO.File]::WriteAllText($sentinel,'preserve')
    $failed=$false
    try{& $entry -Mode Store -Destination $work -Name Work -NonInteractive}catch{$failed=$_.Exception.Message-match'empty'}
    Assert ($failed-and[IO.File]::ReadAllText($sentinel)-ceq'preserve') 'Reinstallation did not preserve existing data'
    Assert ([Net.ServicePointManager]::SecurityProtocol-eq$originalProtocol) 'TLS settings changed'
    $status.checks+='Existing Store refusal preserves data; TLS settings restored'
    $status.status='passed'
}catch{
    $status.status='failed';$status.error=$_.Exception.ToString()
}finally{
    [Environment]::SetEnvironmentVariable('PATH',$originalUserPath,'User')
    $env:PATH=$originalPath;$env:LOCALAPPDATA=$originalLocal
    [Net.ServicePointManager]::SecurityProtocol=$originalProtocol
    Remove-Item Function:\Invoke-RestMethod,Function:\Invoke-WebRequest -ErrorAction SilentlyContinue
    Remove-Variable PortscoopReleaseFixture -Scope Global -ErrorAction SilentlyContinue
    try{
        $after=& (Join-Path $root 'host-state.ps1')
        $after|ConvertTo-Json -Depth 6|Set-Content -LiteralPath (Join-Path $root 'host-after.json') -Encoding UTF8
        Assert (($before|ConvertTo-Json -Depth 6 -Compress)-ceq($after|ConvertTo-Json -Depth 6 -Compress)) 'Host Scoop or persistent environment changed after restoration'
        $status.checks+='Scoop sentinel and user/machine environment unchanged after restoring the test PATH entry'
    }catch{$status.status='failed';$status.error=$_.Exception.ToString()}
    Stop-Transcript -ErrorAction SilentlyContinue|Out-Null
    $status|ConvertTo-Json -Depth 6|Set-Content -LiteralPath $Results -Encoding UTF8
}
if($status.status-ne'passed'){throw $status.error}
$status|ConvertTo-Json -Depth 6
