# run-system.ps1
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "🚀 Starting Cinema Ticket Booking System..." -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# 1. Check if Docker Desktop is running
if (-not (Get-Process -Name "Docker Desktop" -ErrorAction SilentlyContinue)) {
    Write-Host "⚠️ Docker Desktop is not running. Launching program, please wait..." -ForegroundColor Yellow
    Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"
    Start-Sleep -Seconds 10
}

# 2. Shut down existing containers and force clean re-build for Full-Stack app
Write-Host "🛠️ Rebuilding containers (Docker Compose Up)..." -ForegroundColor Green
docker compose down
docker compose up --build

Read-Host -Prompt "Press Enter to close this window"
