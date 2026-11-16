# Build script for upload-s3 Go application
# PowerShell version
# powershell -ExecutionPolicy Bypass -File build.ps1

param(
    [string]$Target = "build",
    [string]$Args = ""
)

$BinaryName = "ajava"
$BinaryWindows = "$BinaryName.exe"
$BinaryLinux = $BinaryName
$SourcePath = "./cmd/ajava"
$BuildDir = ".\bin"

function Show-Help {
    Write-Host "Available targets:"
    Write-Host "  .\build.ps1 deps       - Download and install dependencies"
    Write-Host "  .\build.ps1 build      - Build the application"
    Write-Host "  .\build.ps1 clean      - Clean build artifacts"
    Write-Host "  .\build.ps1 test       - Run tests"
    Write-Host "  .\build.ps1 run        - Run the application"
    Write-Host "  .\build.ps1 linux      - Build for Linux"
    Write-Host "  .\build.ps1 windows    - Build for Windows"
    Write-Host "  .\build.ps1 install    - Install the application"
    Write-Host ""
    Write-Host "Example:"
    Write-Host "  .\build.ps1 run"
}

function Install-Deps {
    Write-Host "Downloading dependencies..."
    go mod download
    go mod tidy
}

function Install-App {
    Write-Host "Installing $BinaryName to c:\green\ajava.exe ..."
    if (-not (Test-Path "c:\green")) {
        New-Item -ItemType Directory -Path "c:\green" | Out-Null
    }
    # Copy the built binary to the installation directory
    # 判断文件是否存在，如果不存在则提示用户先进行构建
    if (-not (Test-Path "$BuildDir\$BinaryWindows")) {
        Invoke-WindowsBuild
    }
    cpFile.bat bin\$BinaryWindows c:\green\ajava.exe
    
}
function Invoke-Build {
    Write-Host "Building $BinaryName..."
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    go build -o "$BuildDir\$BinaryWindows" -v $SourcePath
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build successful: $BuildDir\$BinaryWindows" -ForegroundColor Green
    } else {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-LinuxBuild{
    Write-Host "Building for Linux..."
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -ldflags="-s -w" -o "$BuildDir\$BinaryLinux" -v $SourcePath
    $env:GOOS = ""
    $env:GOARCH = ""
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Linux build successful: $BuildDir\$BinaryLinux" -ForegroundColor Green
    } else {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-WindowsBuild {
    Write-Host "Building for Windows..."
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -ldflags="-s -w" -o "$BuildDir\$BinaryWindows" -v $SourcePath
    $env:GOOS = ""
    $env:GOARCH = ""
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Windows build successful: $BuildDir\$BinaryWindows" -ForegroundColor Green
    } else {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
}

function Clear-Build {
    Write-Host "Cleaning..."
    go clean
    if (Test-Path $BuildDir) {
        Remove-Item -Recurse -Force $BuildDir
        Write-Host "Cleaned build directory" -ForegroundColor Green
    }
}

function Invoke-Tests {
    Write-Host "Running tests..."
    go test -v ./...
}

function Invoke-App {
    Invoke-Build
    Write-Host "Running $BinaryName..."
    if ($Args) {
        & "$BuildDir\$BinaryWindows" $Args.Split(" ")
    } else {
        & "$BuildDir\$BinaryWindows"
    }
}

function Format-Code {
    Write-Host "Formatting code..."
    go fmt ./...
}

function Invoke-CodeVet {
    Write-Host "Running go vet..."
    go vet ./...
}

Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    
    # Main execution
    switch ($Target.ToLower()) {
        "help"      { Show-Help }
        "deps"      { Install-Deps }
        "install"   { Install-App }
        "build"     { Invoke-Build }
        "linux"     { Invoke-LinuxBuild }
        "windows"   { Invoke-WindowsBuild }
        "vet"       { Invoke-CodeVet }
        "test"      { Invoke-Tests }
        "run"       { Invoke-App }
        "fmt"       { Format-Code }
        "clean"     { Clear-Build }
        default {
            Write-Host "Unknown target: $Target" -ForegroundColor Red
            Show-Help
            exit 1
        }
    }
} finally {
    Pop-Location
}

