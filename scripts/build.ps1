# Windows build script — equivalent to `make build` (no make required)
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

New-Item -ItemType Directory -Force -Path "bin" | Out-Null

Write-Host "Building BugIT binaries (Windows)..." -ForegroundColor Cyan

go build -o bin/dre-agent.exe     ./dre-agent/cmd/dre-agent
go build -o bin/dre-collector.exe ./dre-collector/cmd/dre-collector
go build -o bin/dre-cli.exe       ./dre-collector/cmd/dre-cli
go build -o bin/dre-replay.exe    ./dre-replay-cli/cmd/dre-replay
go build -o bin/bugit.exe         ./bugit-cli/cmd/bugit

Write-Host ""
Write-Host "Done. Binaries in bin/" -ForegroundColor Green
Write-Host "  bugit.exe       — plug-and-play capture/replay"
Write-Host "  dre-replay.exe  — replay proxy"
Write-Host ""
Write-Host "Try: .\bin\bugit.exe doctor"
