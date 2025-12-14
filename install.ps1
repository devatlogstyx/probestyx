# Probestyx Windows Installation Script
# Run as Administrator

#Requires -RunAsAdministrator

param(
    [Parameter(Mandatory=$true, Position=0)]
    [string]$ConfigPath
)

$ErrorActionPreference = "Stop"

# Configuration
$GitHubRepo = "devatlogstyx/probestyx"
$InstallDir = "C:\probestyx"
$BinaryName = "probestyx.exe"
$ConfigFile = "config.yaml"
$ServiceName = "Probestyx"

Write-Host "Probestyx Windows Installation Script" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green
Write-Host ""

# Check if config file exists
if (-Not (Test-Path $ConfigPath)) {
    Write-Host "Error: Config file not found: $ConfigPath" -ForegroundColor Red
    Write-Host ""
    Write-Host "Usage:" -ForegroundColor Yellow
    Write-Host '  $config = "C:\path\to\config.yaml"' -ForegroundColor White
    Write-Host '  irm https://raw.githubusercontent.com/.../install.ps1 | iex -ArgumentList $config' -ForegroundColor White
    Write-Host ""
    Write-Host "Example:" -ForegroundColor Yellow
    Write-Host '  irm https://raw.githubusercontent.com/.../install.ps1 | iex -ArgumentList ".\config.yaml"' -ForegroundColor White
    exit 1
}

Write-Host "Using config file: $ConfigPath" -ForegroundColor Green
Write-Host ""

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-Host "Detected Architecture: $Arch" -ForegroundColor Yellow

# Get latest release version
Write-Host "Fetching latest release..." -ForegroundColor Yellow
try {
    $LatestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$GitHubRepo/releases/latest"
    $LatestVersion = $LatestRelease.tag_name
    Write-Host "Latest version: $LatestVersion" -ForegroundColor Green
} catch {
    Write-Host "Error: Could not fetch latest version" -ForegroundColor Red
    exit 1
}

# Download binary
$BinaryUrl = "https://github.com/$GitHubRepo/releases/download/$LatestVersion/probestyx-windows-$Arch.exe"
$TempBinary = "$env:TEMP\probestyx-download.exe"

Write-Host "Downloading from $BinaryUrl..." -ForegroundColor Yellow
try {
    Invoke-WebRequest -Uri $BinaryUrl -OutFile $TempBinary
    Write-Host "Binary downloaded successfully" -ForegroundColor Green
} catch {
    Write-Host "Error: Failed to download binary" -ForegroundColor Red
    exit 1
}

# Check if NSSM is available
$nssmPath = Get-Command nssm -ErrorAction SilentlyContinue

if (-Not $nssmPath) {
    Write-Host "NSSM (Non-Sucking Service Manager) not found" -ForegroundColor Yellow
    Write-Host "Downloading NSSM..." -ForegroundColor Yellow
    
    $nssmZip = "$env:TEMP\nssm.zip"
    $nssmExtract = "$env:TEMP\nssm"
    
    try {
        Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile $nssmZip
        Expand-Archive -Path $nssmZip -DestinationPath $nssmExtract -Force
        
        $nssmArch = if ([Environment]::Is64BitOperatingSystem) { "win64" } else { "win32" }
        Copy-Item "$nssmExtract\nssm-2.24\$nssmArch\nssm.exe" "C:\Windows\System32\" -Force
        
        Remove-Item $nssmZip -Force
        Remove-Item $nssmExtract -Recurse -Force
        
        Write-Host "NSSM installed successfully" -ForegroundColor Green
    } catch {
        Write-Host "Error: Failed to install NSSM" -ForegroundColor Red
        exit 1
    }
}

# Create installation directory
Write-Host "Creating directory $InstallDir..." -ForegroundColor Yellow
if (-Not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

# Check if already installed
$Upgrade = $false
if (Test-Path "$InstallDir\$BinaryName") {
    Write-Host "Probestyx is already installed. Upgrading..." -ForegroundColor Yellow
    $Upgrade = $true
}

# Install binary
Write-Host "Installing binary..." -ForegroundColor Yellow
Copy-Item $TempBinary "$InstallDir\$BinaryName" -Force

# Install config
if ($Upgrade) {
    Write-Host "Upgrading: Replacing config file with your new config..." -ForegroundColor Yellow
    Write-Host "(Your old config will be backed up)" -ForegroundColor Yellow
    if (Test-Path "$InstallDir\$ConfigFile") {
        $timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
        Copy-Item "$InstallDir\$ConfigFile" "$InstallDir\$ConfigFile.backup.$timestamp" -Force
    }
}

Write-Host "Copying config file..." -ForegroundColor Yellow
Copy-Item $ConfigPath "$InstallDir\$ConfigFile" -Force

# Check if service already exists
$existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue

if ($existingService -and -not $Upgrade) {
    Write-Host "Service already exists, removing..." -ForegroundColor Yellow
    nssm stop $ServiceName
    nssm remove $ServiceName confirm
    Start-Sleep -Seconds 2
}

if ($Upgrade -and $existingService) {
    # Just restart the service for upgrade
    Write-Host "Restarting service after upgrade..." -ForegroundColor Yellow
    nssm restart $ServiceName
    Start-Sleep -Seconds 2
} else {
    # Install service with NSSM
    Write-Host "Installing Windows service..." -ForegroundColor Yellow

    nssm install $ServiceName "$InstallDir\$BinaryName" "$InstallDir\$ConfigFile"
    nssm set $ServiceName AppDirectory $InstallDir
    nssm set $ServiceName DisplayName "Probestyx Metrics Collection"
    nssm set $ServiceName Description "Collects and aggregates system and application metrics"
    nssm set $ServiceName Start SERVICE_AUTO_START
    nssm set $ServiceName AppStdout "$InstallDir\probestyx.log"
    nssm set $ServiceName AppStderr "$InstallDir\probestyx.log"
    nssm set $ServiceName AppRotateFiles 1
    nssm set $ServiceName AppRotateBytes 10485760  # 10MB

    # Start service
    Write-Host "Starting service..." -ForegroundColor Yellow
    nssm start $ServiceName

    Start-Sleep -Seconds 2
}

# Check service status
$status = nssm status $ServiceName

# Cleanup temp files
Remove-Item $TempBinary -Force -ErrorAction SilentlyContinue

Write-Host ""
if ($Upgrade) {
    Write-Host "Upgrade complete!" -ForegroundColor Green
} else {
    Write-Host "Installation complete!" -ForegroundColor Green
}
Write-Host "Installed version: $LatestVersion" -ForegroundColor Cyan
Write-Host "Config location: $InstallDir\$ConfigFile" -ForegroundColor Cyan
Write-Host "Service status: $status" -ForegroundColor Cyan
Write-Host ""
Write-Host "Service is running on http://localhost:9100" -ForegroundColor Cyan
Write-Host "Test with: Invoke-WebRequest http://localhost:9100/metrics" -ForegroundColor Cyan
Write-Host ""
Write-Host "Manage the service with:" -ForegroundColor Yellow
Write-Host "  nssm status $ServiceName" -ForegroundColor White
Write-Host "  nssm restart $ServiceName" -ForegroundColor White
Write-Host "  nssm stop $ServiceName" -ForegroundColor White
Write-Host "  Get-Content $InstallDir\probestyx.log -Tail 50 -Wait" -ForegroundColor White