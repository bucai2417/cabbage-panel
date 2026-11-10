param([string]$TargetOS = 'windows', [string]$TargetArch = 'amd64', [switch]$SkipWeb)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $projectRoot
if (-not (Get-Command go -ErrorAction SilentlyContinue) -and (Test-Path -LiteralPath 'D:\Go\bin\go.exe')) { $env:PATH = 'D:\Go\bin;' + $env:PATH }
New-Item -ItemType Directory -Force -Path '.tools\gopath', '.tools\gocache', '.tools\tmp', '.tools\npm-cache', 'dist' | Out-Null
$env:GOPATH = Join-Path $projectRoot '.tools\gopath'
$env:GOCACHE = Join-Path $projectRoot '.tools\gocache'
$env:GOTMPDIR = Join-Path $projectRoot '.tools\tmp'
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:GOTMPDIR
$env:npm_config_cache = Join-Path $projectRoot '.tools\npm-cache'
if (-not $SkipWeb) {
    npm.cmd --prefix web ci
    if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
    npm.cmd --prefix web run build
    if ($LASTEXITCODE -ne 0) { throw 'frontend build failed' }
}
$releaseVersion = (Get-Content -LiteralPath 'VERSION' -Raw -Encoding UTF8).Trim()
$env:CGO_ENABLED = '0'
$env:GOOS = $TargetOS
$env:GOARCH = $TargetArch
$outputName = "dist/cabbage-$TargetOS-$TargetArch"
if ($TargetOS -eq 'windows') { $outputName += '.exe' }
$flags = "-s -w -X main.version=$releaseVersion"
if ($env:CABBAGE_UPDATE_PUBLIC_KEY) { $flags += " -X main.updateKey=$env:CABBAGE_UPDATE_PUBLIC_KEY" }
go build -trimpath -buildvcs=false -ldflags $flags -o $outputName ./cmd/cabbage
if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
Write-Output "Built $outputName (v$releaseVersion)"
