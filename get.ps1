# Portscoop's small internet entry point. Installer helpers run in a child scope.
[CmdletBinding()]
param(
    [string]$Mode,
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
& {
    param($Options)
    $ErrorActionPreference='Stop'
    $ref='main'
    if ($Options.Version) {
        if ($Options.Version -notmatch '^v?[0-9][0-9A-Za-z._+-]*$') { throw "Invalid release version: $($Options.Version)" }
        $ref=if($Options.Version.StartsWith('v')){$Options.Version}else{'v'+$Options.Version}
    }
    $uri='https://raw.githubusercontent.com/ZackaryW/portable-scoop/'+$ref+'/scripts/install.ps1'
    $previousProtocol=[Net.ServicePointManager]::SecurityProtocol
    try {
        [Net.ServicePointManager]::SecurityProtocol=$previousProtocol -bor [Net.SecurityProtocolType]::Tls12
        $source=Invoke-RestMethod -Uri $uri -TimeoutSec 60
        if ($source -isnot [string] -or [string]::IsNullOrWhiteSpace($source)) { throw 'GitHub did not return an installer script' }
        & ([scriptblock]::Create($source)) @Options
    } finally { [Net.ServicePointManager]::SecurityProtocol=$previousProtocol }
} $PSBoundParameters
