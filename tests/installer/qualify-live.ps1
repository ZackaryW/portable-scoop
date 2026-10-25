# Disposable VMware guest only. All installer and release requests use GitHub.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Sentinel,
    [Parameter(Mandatory=$true)][string]$Results,
    [string]$ReleaseVersion='0.1.0'
)
$ErrorActionPreference='Stop'
$root=Join-Path 'C:\Users\Public' ('PortscoopLive-'+[guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $root|Out-Null
$originalPath=$env:PATH;$originalLocal=$env:LOCALAPPDATA
$originalUserPath=[Environment]::GetEnvironmentVariable('PATH','User')
$originalProtocol=[Net.ServicePointManager]::SecurityProtocol
$status=[ordered]@{status='running';root=$root;release=$ReleaseVersion;windows=[Environment]::OSVersion.Version.ToString();powershell=$PSVersionTable.PSVersion.ToString();go_absent=(-not[bool](Get-Command go -ErrorAction SilentlyContinue));checks=@();error=$null}
function Assert([bool]$Condition,[string]$Message){if(-not$Condition){throw $Message}}
Start-Transcript -Path (Join-Path $root 'live-install.log')|Out-Null
try {
    $before=& $Sentinel
    $before|ConvertTo-Json -Depth 6|Set-Content (Join-Path $root 'host-before.json') -Encoding UTF8
    $env:LOCALAPPDATA=Join-Path $root 'host-profile'
    New-Item -ItemType Directory -Path $env:LOCALAPPDATA|Out-Null
    $hostRoot=Join-Path $root 'Host Launcher'
    $global:PortscoopLiveAnswers=New-Object 'Collections.Generic.Queue[string]'
    foreach($answer in @('1',$hostRoot,'y')){$global:PortscoopLiveAnswers.Enqueue($answer)}
    # Supply console answers only; download functions remain the real cmdlets.
    function global:Read-Host {
        param([string]$Prompt)
        if(-not$global:PortscoopLiveAnswers.Count){throw ('Unexpected wizard prompt: '+$Prompt)}
        $global:PortscoopLiveAnswers.Dequeue()
    }
    & {Invoke-RestMethod -Uri https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/get.ps1|Invoke-Expression}
    Remove-Item Function:\Read-Host
    Assert ($global:PortscoopLiveAnswers.Count-eq 0) 'Wizard did not consume expected answers'
    $listing=psp env list|Out-String
    Assert ($LASTEXITCODE-eq 0-and$listing-match'ALIAS'-and$env:PATH.StartsWith($hostRoot+';')) 'Bare host launcher unavailable after live wizard'
    $expectedPath=$originalUserPath
    if($expectedPath-and-not$expectedPath.EndsWith(';')){$expectedPath+=';'}
    $expectedPath+=$hostRoot
    Assert ([Environment]::GetEnvironmentVariable('PATH','User')-ceq$expectedPath) 'Unexpected user PATH modification'
    $versionOutput=psp --version|Out-String
    Assert ($LASTEXITCODE-eq 0-and$versionOutput.Contains($ReleaseVersion)) 'Latest release version differs'
    $status.checks+='Live main/get.ps1 IEX wizard, latest ZIP/checksum, exact host PATH entry and immediate psp env list'

    $store=Join-Path $root 'Store (portable)'
    $get=Invoke-RestMethod -Uri ('https://raw.githubusercontent.com/ZackaryW/portable-scoop/v'+$ReleaseVersion+'/get.ps1')
    $hostPath=$env:PATH
    & ([scriptblock]::Create($get)) -Mode Store -Destination $store -Name 'Live Work' -Version $ReleaseVersion -RegisterAs work -SetDefault -NonInteractive
    Assert ($env:PATH-ceq$hostPath) 'Store changed caller PATH'
    foreach($exe in @((Join-Path $store 'psp.exe'),(Join-Path $hostRoot 'psp.exe'))){
        $output=& $exe list|Out-String
        Assert ($LASTEXITCODE-eq 1-and$output-match"There aren't any apps installed") 'Expected empty Store backend response'
    }
    $listing=psp env list|Out-String
    Assert ($LASTEXITCODE-eq 0-and$listing-match'work'-and$listing-match'available') 'Registered default unavailable'
    [IO.File]::WriteAllText((Join-Path $root 'env-list.txt'),$listing)
    $status.checks+='Live tagged entry/worker/release, Store initialization, explicit registration/default, direct execution and central delegation'
    Assert ([Net.ServicePointManager]::SecurityProtocol-eq$originalProtocol) 'Installer changed TLS settings'
    $status.status='passed'
}catch{$status.status='failed';$status.error=$_.Exception.ToString()}
finally{
    [Environment]::SetEnvironmentVariable('PATH',$originalUserPath,'User')
    $env:PATH=$originalPath;$env:LOCALAPPDATA=$originalLocal
    [Net.ServicePointManager]::SecurityProtocol=$originalProtocol
    Remove-Item Function:\Read-Host -ErrorAction SilentlyContinue
    Remove-Variable PortscoopLiveAnswers -Scope Global -ErrorAction SilentlyContinue
    try{
        $after=& $Sentinel
        $after|ConvertTo-Json -Depth 6|Set-Content (Join-Path $root 'host-after.json') -Encoding UTF8
        Assert (($before|ConvertTo-Json -Depth 6 -Compress)-ceq($after|ConvertTo-Json -Depth 6 -Compress)) 'Scoop sentinel or persistent environment changed'
        $status.checks+='Scoop sentinel and user/machine environment unchanged after restoring test PATH'
    }catch{$status.status='failed';$status.error=$_.Exception.ToString()}
    Stop-Transcript|Out-Null
    $status|ConvertTo-Json -Depth 6|Set-Content -LiteralPath $Results -Encoding UTF8
}
if($status.status-ne'passed'){throw $status.error}
$status|ConvertTo-Json -Depth 6
