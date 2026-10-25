function Read-PortscoopRequest {
    $ErrorActionPreference = 'Stop'
    $root = [IO.Path]::GetFullPath((Split-Path -Parent (Split-Path -Parent $PSScriptRoot)))
    if (-not $env:PORTSCOOP_ROOT -or -not $root.Equals([IO.Path]::GetFullPath($env:PORTSCOOP_ROOT), [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Portable root does not match the fixed backend location'
    }
    $requestPath = [IO.Path]::GetFullPath($env:PORTSCOOP_REQUEST_FILE)
    $requestDirectory = Join-Path $root 'scoop\tmp'
    if (-not ([IO.Path]::GetDirectoryName($requestPath)).Equals($requestDirectory, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Request transport is outside the portable temporary directory'
    }
    $marker = Get-Content -LiteralPath (Join-Path $root '.portscoop\environment.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    $request = Get-Content -LiteralPath $requestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($request.schema_version -ne 1 -or $marker.schema_version -ne 1 -or $marker.protocol_major -ne 1 -or $marker.ready -ne $true -or $request.expected_id -cne $marker.id) {
        throw 'Request schema or portable identity changed before backend execution'
    }
    if ($request.arguments -isnot [Array]) { throw 'Request arguments must be an array' }
    foreach ($argument in $request.arguments) {
        if ($argument -isnot [string] -or $argument.IndexOf([char]0) -ge 0) { throw 'Invalid request argument' }
    }
    return $request
}
