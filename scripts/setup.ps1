#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][ValidateSet('Central','Portable')][string]$Mode,
    [Parameter(Mandatory=$true)][string]$Destination,
    [string]$Name,
    [switch]$AddToPath
)
$ErrorActionPreference='Stop'
if ($Mode -eq 'Portable' -and $AddToPath) { throw 'AddToPath is available only for explicit central setup' }
if ($Mode -eq 'Portable' -and [string]::IsNullOrWhiteSpace($Name)) { throw 'Portable setup requires -Name' }
$source=[IO.Path]::GetFullPath($PSScriptRoot)
$Destination=[IO.Path]::GetFullPath($Destination)
& (Join-Path $source 'portscoop.exe') __portscoop_verify_release
if ($LASTEXITCODE -ne 0) { throw 'Source release verification failed' }
if ($Destination -ine $source) {
    if ((Test-Path -LiteralPath $Destination) -and @(Get-ChildItem -LiteralPath $Destination -Force).Count -gt 0) { throw 'Choose an empty setup destination; existing files were preserved' }
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    Get-ChildItem -LiteralPath $source -Force | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination $Destination -Recurse }
}
& (Join-Path $Destination 'portscoop.exe') __portscoop_verify_release
if ($LASTEXITCODE -ne 0) { throw 'Copied release verification failed' }
if ($Mode -eq 'Portable') {
    & (Join-Path $Destination 'portscoop.exe') env init $Destination --name $Name
    if ($LASTEXITCODE -ne 0) { throw 'Portable initialization failed; preserve the destination for inspection' }
} elseif ($AddToPath) {
    $userPath=[Environment]::GetEnvironmentVariable('PATH','User')
    $parts=@($userPath -split ';' | Where-Object { $_ })
    if (-not @($parts | Where-Object { $_.TrimEnd('\') -ieq $Destination.TrimEnd('\') }).Count) {
        $updatedPath=$userPath
        if ($updatedPath -and -not $updatedPath.EndsWith(';')) { $updatedPath+=';' }
        $updatedPath+=$Destination
        [Environment]::SetEnvironmentVariable('PATH',$updatedPath,'User')
    }
    Write-Output 'Central launcher added to user PATH. Open a new terminal to use it.'
}
Write-Output "Portscoop $Mode setup completed at $Destination"
