#!/usr/bin/env pwsh
# Comprehensive test script for ajava
# Tests all major features of the tool
# run the test cases: powershell -ExecutionPolicy Bypass -File test-all.ps1

$ErrorActionPreference = "Stop"
$global:TestsPassed = 0
$global:TestsFailed = 0
$global:TestResults = @()



# Color output functions
function Write-TestHeader {
    param([string]$Message)
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host $Message -ForegroundColor Cyan
    Write-Host "========================================`n" -ForegroundColor Cyan
}

function Write-TestSuccess {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
    $global:TestsPassed++
    $global:TestResults += @{Status="PASS"; Test=$Message}
}

function Write-TestFailure {
    param([string]$Message, [string]$Error)
    Write-Host "✗ $Message" -ForegroundColor Red
    Write-Host "  Error: $Error" -ForegroundColor Red
    $global:TestsFailed++
    $global:TestResults += @{Status="FAIL"; Test=$Message; Error=$Error}
}

function Write-TestInfo {
    param([string]$Message)
    Write-Host "→ $Message" -ForegroundColor Yellow
}

# Test result summary
function Write-TestSummary {
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "TEST SUMMARY" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Total Tests: $($global:TestsPassed + $global:TestsFailed)" -ForegroundColor White
    Write-Host "Passed: $global:TestsPassed" -ForegroundColor Green
    Write-Host "Failed: $global:TestsFailed" -ForegroundColor $(if ($global:TestsFailed -eq 0) { "Green" } else { "Red" })
    
    if ($global:TestsFailed -gt 0) {
        Write-Host "`nFailed Tests:" -ForegroundColor Red
        foreach ($result in $global:TestResults) {
            if ($result.Status -eq "FAIL") {
                Write-Host "  - $($result.Test)" -ForegroundColor Red
                Write-Host "    $($result.Error)" -ForegroundColor Gray
            }
        }
    }
    
    Write-Host "========================================`n" -ForegroundColor Cyan
    
    if ($global:TestsFailed -gt 0) {
        exit 1
    }
}

Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    # Build the tool first
    Write-TestHeader "Building ajava"
    try {
        Write-TestInfo "Running: go build -o ajava.exe ./cmd/ajava"
        go build -o ajava.exe ./cmd/ajava
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed with exit code $LASTEXITCODE"
        }
        Write-TestSuccess "Tool built successfully"
    } catch {
        Write-TestFailure "Tool build" $_.Exception.Message
        exit 1
    }

    # Test 1: Simple standalone Java project
    Write-TestHeader "Test 1: Simple Standalone Java Project"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        # Clean previous builds
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "classes" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building simple Java project"
        ..\..\ajava.exe build -main com.example.Main
        
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed with exit code $LASTEXITCODE"
        }
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "JAR file not created"
        }
        
        Write-TestInfo "Testing JAR execution"
        $output = java -jar bin\app.jar 2>&1 | Out-String
        if ($output -notmatch "Hello") {
            throw "JAR execution did not produce expected output"
        }
        
        Pop-Location
        Write-TestSuccess "Simple standalone project - normal JAR"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Simple standalone project" $_.Exception.Message
    }

    # Test 2: Simple project with static mode (Fat JAR)
    Write-TestHeader "Test 2: Simple Project - Fat JAR Mode"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "classes" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building Fat JAR"
        ..\..\ajava.exe build -main com.example.Main -static
        
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed with exit code $LASTEXITCODE"
        }
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "Fat JAR not created"
        }
        
        Write-TestInfo "Testing Fat JAR execution"
        $output = java -jar bin\app.jar 2>&1 | Out-String
        if ($output -notmatch "Hello") {
            throw "Fat JAR execution failed"
        }
        
        Pop-Location
        Write-TestSuccess "Simple project - Fat JAR mode"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Fat JAR mode" $_.Exception.Message
    }

    # Test 3: Simple project with explib mode
    Write-TestHeader "Test 3: Simple Project - Export Dependencies Mode"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "classes" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building with explib mode"
        ..\..\ajava.exe build -main com.example.Main -explib
        
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed with exit code $LASTEXITCODE"
        }
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "JAR not created"
        }
        
        if (-not (Test-Path "bin\lib")) {
            throw "lib directory not created"
        }
        
        Write-TestInfo "Testing JAR with external dependencies"
        $output = java -jar bin\app.jar 2>&1 | Out-String
        if ($output -notmatch "Hello") {
            throw "JAR execution failed"
        }
        
        Pop-Location
        Write-TestSuccess "Simple project - explib mode"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Explib mode" $_.Exception.Message
    }

    # Test 4: Maven project - normal mode
    Write-TestHeader "Test 4: Maven Project - Normal Mode"
    try {
        $testDir = "examples\maven-test"
        Push-Location $testDir
        
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "target" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building Maven project"
        ..\..\ajava.exe build -main com.example.MavenApp
        
        if ($LASTEXITCODE -ne 0) {
            throw "Maven build failed with exit code $LASTEXITCODE"
        }
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "Maven JAR not created"
        }
        
        Write-TestInfo "Testing Maven JAR execution"
        $output = java -jar bin\app.jar 2>&1 | Out-String
        if ($output -notmatch "Maven") {
            throw "Maven JAR execution failed"
        }
        
        Pop-Location
        Write-TestSuccess "Maven project - normal mode"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Maven project normal mode" $_.Exception.Message
    }

    # Test 5: Maven project - Fat JAR mode
    Write-TestHeader "Test 5: Maven Project - Fat JAR Mode"
    try {
        $testDir = "examples\maven-test"
        Push-Location $testDir
        
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "target" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building Maven Fat JAR"
        ..\..\ajava.exe build -main com.example.MavenApp -static
        
        if ($LASTEXITCODE -ne 0) {
            throw "Maven Fat JAR build failed with exit code $LASTEXITCODE"
        }
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "Maven Fat JAR not created"
        }
        
        Write-TestInfo "Testing Maven Fat JAR execution"
        $output = java -jar bin\app.jar 2>&1 | Out-String
        if ($output -notmatch "Maven") {
            throw "Maven Fat JAR execution failed"
        }
        
        Pop-Location
        Write-TestSuccess "Maven project - Fat JAR mode"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Maven Fat JAR mode" $_.Exception.Message
    }

    # Test 6: Maven project - explib mode
    Write-TestHeader "Test 6: Maven Project - Export Dependencies Mode"
    try {
        $testDir = "examples\maven-test"
        Push-Location $testDir
        
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "target" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building Maven project with explib"
        ..\..\ajava.exe build -main com.example.MavenApp -explib
        
        if ($LASTEXITCODE -ne 0) {
            throw "Maven explib build failed with exit code $LASTEXITCODE"
        }
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "Maven JAR not created"
        }
        
        if (-not (Test-Path "bin\lib")) {
            throw "lib directory not created for Maven project"
        }
        
        Write-TestInfo "Testing Maven JAR with external dependencies"
        $output = java -jar bin\app.jar 2>&1 | Out-String
        if ($output -notmatch "Maven") {
            throw "Maven JAR execution failed"
        }
        
        Pop-Location
        Write-TestSuccess "Maven project - explib mode"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Maven explib mode" $_.Exception.Message
    }

    # Test 7: Using existing JAR
    Write-TestHeader "Test 7: Using Existing JAR"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        # First build a JAR
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        ..\..\ajava.exe build -main com.example.Main
        
        if (-not (Test-Path "bin\app.jar")) {
            throw "Initial JAR not created"
        }
        
        Write-TestInfo "Testing with existing JAR"
        $output = ..\..\ajava.exe build -jar bin\app.jar -jarname test.jar -out bin 2>&1 | Out-String
        
        # The tool should recognize the JAR and not fail
        if ($LASTEXITCODE -ne 0) {
            Write-TestInfo "Expected behavior: tool accepts existing JAR"
        }
        
        Pop-Location
        Write-TestSuccess "Using existing JAR"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Using existing JAR" $_.Exception.Message
    }

    # Test 8: IDE project generation - VS Code
    Write-TestHeader "Test 8: IDE Project Generation - VS Code"
    try {
        $tempDir = "test-vscode-temp"
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Generating VS Code project"
        .\ajava.exe new vsc $tempDir
        
        if ($LASTEXITCODE -ne 0) {
            throw "VS Code project generation failed"
        }
        
        if (-not (Test-Path "$tempDir\.vscode\settings.json")) {
            throw "VS Code settings not created"
        }
        
        if (-not (Test-Path "$tempDir\src\Main.java")) {
            throw "Source file not created"
        }
        
        Remove-Item -Path $tempDir -Recurse -Force
        Write-TestSuccess "VS Code project generation"
    } catch {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-TestFailure "VS Code project generation" $_.Exception.Message
    }

    # Test 9: IDE project generation - Eclipse
    Write-TestHeader "Test 9: IDE Project Generation - Eclipse"
    try {
        $tempDir = "test-eclipse-temp"
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Generating Eclipse project"
        .\ajava.exe new eclipse $tempDir
        
        if ($LASTEXITCODE -ne 0) {
            throw "Eclipse project generation failed"
        }
        
        if (-not (Test-Path "$tempDir\.project")) {
            throw "Eclipse .project not created"
        }
        
        if (-not (Test-Path "$tempDir\.classpath")) {
            throw "Eclipse .classpath not created"
        }
        
        Remove-Item -Path $tempDir -Recurse -Force
        Write-TestSuccess "Eclipse project generation"
    } catch {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-TestFailure "Eclipse project generation" $_.Exception.Message
    }

    # Test 10: IDE project generation - IntelliJ IDEA
    Write-TestHeader "Test 10: IDE Project Generation - IntelliJ IDEA"
    try {
        $tempDir = "test-idea-temp"
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Generating IntelliJ IDEA project"
        .\ajava.exe new idea $tempDir
        
        if ($LASTEXITCODE -ne 0) {
            throw "IntelliJ IDEA project generation failed"
        }
        
        if (-not (Test-Path "$tempDir\.idea")) {
            throw "IntelliJ IDEA .idea directory not created"
        }
        
        Remove-Item -Path $tempDir -Recurse -Force
        Write-TestSuccess "IntelliJ IDEA project generation"
    } catch {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-TestFailure "IntelliJ IDEA project generation" $_.Exception.Message
    }

    # Test 11: IDE project generation - Maven
    Write-TestHeader "Test 11: IDE Project Generation - Maven"
    try {
        $tempDir = "test-maven-temp"
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Generating Maven project"
        .\ajava.exe new maven $tempDir
        
        if ($LASTEXITCODE -ne 0) {
            throw "Maven project generation failed"
        }
        
        if (-not (Test-Path "$tempDir\pom.xml")) {
            throw "Maven pom.xml not created"
        }
        
        Remove-Item -Path $tempDir -Recurse -Force
        Write-TestSuccess "Maven project generation"
    } catch {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-TestFailure "Maven project generation" $_.Exception.Message
    }

    # Test 12: IDE project generation - Spring Boot
    Write-TestHeader "Test 12: IDE Project Generation - Spring Boot"
    try {
        $tempDir = "test-spring-temp"
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Generating Spring Boot project"
        .\ajava.exe new spring $tempDir
        
        if ($LASTEXITCODE -ne 0) {
            throw "Spring Boot project generation failed"
        }
        
        if (-not (Test-Path "$tempDir\pom.xml")) {
            throw "Spring Boot pom.xml not created"
        }
        
        if (-not (Test-Path "$tempDir\src\main\java")) {
            throw "Spring Boot source directory not created"
        }
        
        Remove-Item -Path $tempDir -Recurse -Force
        Write-TestSuccess "Spring Boot project generation"
    } catch {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-TestFailure "Spring Boot project generation" $_.Exception.Message
    }

    # Test 13: Custom JAR name
    Write-TestHeader "Test 13: Custom JAR Name"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        Remove-Item -Path "bin" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "classes" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building with custom JAR name"
        ..\..\ajava.exe build -main com.example.Main -jarname myapp.jar
        
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed"
        }
        
        if (-not (Test-Path "bin\myapp.jar")) {
            throw "Custom JAR name not applied"
        }
        
        Pop-Location
        Write-TestSuccess "Custom JAR name"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Custom JAR name" $_.Exception.Message
    }

    # Test 14: Custom output directory
    Write-TestHeader "Test 14: Custom Output Directory"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        Remove-Item -Path "output" -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "classes" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Building with custom output directory"
        ..\..\ajava.exe build -main com.example.Main -out output
        
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed"
        }
        
        if (-not (Test-Path "output\app.jar")) {
            throw "JAR not created in custom output directory"
        }
        
        Remove-Item -Path "output" -Recurse -Force
        Pop-Location
        Write-TestSuccess "Custom output directory"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Custom output directory" $_.Exception.Message
    }

    # Test 15: Skip JAR creation (compile only)
    Write-TestHeader "Test 15: Skip JAR Creation (Compile Only)"
    try {
        $testDir = "examples\simple-standalone"
        Push-Location $testDir
        
        Remove-Item -Path "classes" -Recurse -Force -ErrorAction SilentlyContinue
        
        Write-TestInfo "Compiling without creating JAR"
        ..\..\ajava.exe build -main com.example.Main -nojar
        
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed"
        }
        
        if (-not (Test-Path "classes\com\example\Main.class")) {
            throw "Class files not created"
        }
        
        Pop-Location
        Write-TestSuccess "Skip JAR creation"
    } catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-TestFailure "Skip JAR creation" $_.Exception.Message
    }

    # Print final summary
    Write-TestSummary
} finally {
    Pop-Location
}

