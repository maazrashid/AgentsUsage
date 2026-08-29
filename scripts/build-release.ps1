[CmdletBinding()]
param(
    [ValidateSet('current', 'all', 'windows-amd64', 'linux-amd64', 'darwin-amd64', 'darwin-arm64')]
    [string[]]$Target = @('current'),

    [string]$OutputDirectory = 'dist'
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$outputRoot = if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory
} else {
    Join-Path $repositoryRoot $OutputDirectory
}

$targetTable = @{
    'windows-amd64' = @{ OS = 'windows'; Arch = 'amd64'; Extension = '.exe'; Archive = 'zip' }
    'linux-amd64'   = @{ OS = 'linux'; Arch = 'amd64'; Extension = ''; Archive = 'tar.gz' }
    'darwin-amd64'  = @{ OS = 'darwin'; Arch = 'amd64'; Extension = ''; Archive = 'tar.gz' }
    'darwin-arm64'  = @{ OS = 'darwin'; Arch = 'arm64'; Extension = ''; Archive = 'tar.gz' }
}

if ($Target -contains 'all') {
    $Target = @('windows-amd64', 'linux-amd64', 'darwin-amd64', 'darwin-arm64')
} elseif ($Target -contains 'current') {
    $currentOS = (& go env GOOS).Trim()
    $currentArch = (& go env GOARCH).Trim()
    $currentKey = "$currentOS-$currentArch"
    if (-not $targetTable.ContainsKey($currentKey)) {
        throw "The current Go target $currentKey is not a supported release target."
    }
    $Target = @($currentKey)
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
$stageRoot = Join-Path $outputRoot ".stage-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $stageRoot | Out-Null
$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH
$originalCGOEnabled = $env:CGO_ENABLED

try {
    foreach ($targetName in $Target) {
        $targetConfig = $targetTable[$targetName]
        $stageDirectory = Join-Path $stageRoot $targetName
        New-Item -ItemType Directory -Path $stageDirectory | Out-Null

        $binaryName = "AgentsUsage$($targetConfig.Extension)"
        $binaryPath = Join-Path $stageDirectory $binaryName
        $linkerFlags = '-s -w'
        if ($targetConfig.OS -eq 'windows') {
            $linkerFlags += ' -H=windowsgui'
        }

        $env:GOOS = $targetConfig.OS
        $env:GOARCH = $targetConfig.Arch
        $env:CGO_ENABLED = '0'
        & go build -trimpath -buildvcs=false "-ldflags=$linkerFlags" -o $binaryPath ./cmd/agentsusage
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed for $targetName."
        }

        $archiveBase = Join-Path $outputRoot "AgentsUsage-$targetName"
        if ($targetConfig.Archive -eq 'zip') {
            $archivePath = "$archiveBase.zip"
            Compress-Archive -LiteralPath $binaryPath -DestinationPath $archivePath -Force
        } else {
            $archivePath = "$archiveBase.tar.gz"
            & tar -C $stageDirectory -czf $archivePath $binaryName
            if ($LASTEXITCODE -ne 0) {
                throw "Archive creation failed for $targetName."
            }
        }
        Write-Host "Created $archivePath"
    }

    $checksumPath = Join-Path $outputRoot 'SHA256SUMS'
    $checksumLines = Get-ChildItem -LiteralPath $outputRoot -File |
        Where-Object { $_.Name -like 'AgentsUsage-*' } |
        Sort-Object Name |
        ForEach-Object {
            $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
            "$hash  $($_.Name)"
        }
    Set-Content -LiteralPath $checksumPath -Value $checksumLines -Encoding utf8NoBOM
    Write-Host "Created $checksumPath"
} finally {
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    $env:CGO_ENABLED = $originalCGOEnabled
    if (Test-Path -LiteralPath $stageRoot) {
        Remove-Item -LiteralPath $stageRoot -Recurse -Force
    }
}
