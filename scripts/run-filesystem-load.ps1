[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$BaseUrl,

    [Parameter(Mandatory = $true)]
    [string]$WorkspaceId,

    [Parameter(Mandatory = $true)]
    [string]$Token,

    [string]$ProviderId = $env:FS_LOAD_PROVIDER,

    [ValidateRange(1, 200)]
    [int]$ConcurrentTransfers = 200,

    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$LargeFilePath,

    [ValidateRange(1, 1024)]
    [int]$PartSizeMiB = 64,

    [ValidateRange(100, 1000000)]
    [int]$MetadataRows = 10000,

    [string]$ReportBase = "artifacts/filesystem-load-report"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $repoRoot "backend"
$report = Join-Path $repoRoot $ReportBase
$fixture = (Resolve-Path -LiteralPath $LargeFilePath).Path
$fixtureSize = (Get-Item -LiteralPath $fixture).Length
$expectedSize = 20GB

if ([string]::IsNullOrWhiteSpace($ProviderId)) {
    throw "ProviderId is required. Pass -ProviderId or set FS_LOAD_PROVIDER."
}
if ($fixtureSize -ne $expectedSize) {
    throw "LargeFilePath must be exactly 20 GiB ($expectedSize bytes); got $fixtureSize bytes."
}

$saved = @{}
$variables = @{
    FS_LOAD_RUN = "1"
    FS_LOAD_BASE_URL = $BaseUrl.TrimEnd("/")
    FS_LOAD_WORKSPACE = $WorkspaceId
    FS_LOAD_TOKEN = $Token
    FS_LOAD_PROVIDER = $ProviderId
    FS_LOAD_CONCURRENCY = [string]$ConcurrentTransfers
    FS_LOAD_LARGE_FILE = $fixture
    FS_LOAD_PART_SIZE_MIB = [string]$PartSizeMiB
    FS_LOAD_METADATA_ROWS = [string]$MetadataRows
    FS_LOAD_METADATA_PAGES = "100"
    FS_LOAD_REPORT = $report
}

try {
    foreach ($entry in $variables.GetEnumerator()) {
        $saved[$entry.Key] = [Environment]::GetEnvironmentVariable($entry.Key, "Process")
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, "Process")
    }

    Remove-Item -LiteralPath "$report.json", "$report.md" -Force -ErrorAction SilentlyContinue
    Push-Location $backend
    try {
        go test ./tests/filesystem/performance -count=1 -timeout 8h
        if ($LASTEXITCODE -ne 0) {
            throw "Filesystem load tests failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }

    Write-Host "Load report: $report.md"
    Write-Host "Machine-readable report: $report.json"
}
finally {
    foreach ($entry in $variables.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable($entry.Key, $saved[$entry.Key], "Process")
    }
}
