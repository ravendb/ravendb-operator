# Build and push release images to DockerHub
# Usage: .\release.ps1 [-Version "4.2.0"] [-DryRun]

param(
    [string]$Version,  # Optional - defaults to VERSION in Makefile
    [switch]$DryRun    # Build only, skip pushing
)

$ErrorActionPreference = "Stop"

$versionArg = if ($Version) { "VERSION=$Version" } else { "" }

Write-Host "=== Building release images ===" -ForegroundColor Cyan
make docker-build-release $versionArg
if ($LASTEXITCODE -ne 0) { exit 1 }

if ($DryRun) {
    Write-Host ""
    Write-Host "=== Dry run - skipping push ===" -ForegroundColor Yellow
    exit 0
}

Write-Host ""
Write-Host "=== Pushing version tag ===" -ForegroundColor Cyan
make docker-push-version $versionArg
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host ""
Write-Host "=== Pushing latest tag ===" -ForegroundColor Cyan
make docker-push-latest
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host ""
Write-Host "=== Release complete ===" -ForegroundColor Green

