@echo off
setlocal enabledelayedexpansion

set APP_NAME=app
set VERSION=1.0.0

set DIST_DIR=dist
set PACKAGE_DIR=%DIST_DIR%\%APP_NAME%

echo =========================================
echo Building %APP_NAME% Distribution Package
echo =========================================

echo.
echo Cleaning old builds...
rmdir /s /q %DIST_DIR% 2>nul
mkdir %DIST_DIR%
mkdir %PACKAGE_DIR%
mkdir %PACKAGE_DIR%\config

call setup-jdk25.bat

echo.
echo =========================================
echo Step 1/4 - Building Spring Boot JAR
echo =========================================
call mvnw.cmd clean package -DskipTests

if errorlevel 1 (
    echo ERROR: Maven build failed!
    pause
    exit /b 1
)

echo.
echo =========================================
echo Step 2/4 - Copying JAR file
echo =========================================

for %%f in (target\*.jar) do (
    set JAR=%%f
    set JAR_NAME=%%~nxf
)

if not defined JAR_NAME (
    echo ERROR: No JAR file found in target directory!
    pause
    exit /b 1
)

echo Copying %JAR_NAME% to package directory...
copy /Y "%JAR%" "%PACKAGE_DIR%\%JAR_NAME%"

echo.
echo =========================================
echo Step 3/4 - Creating launcher scripts
echo =========================================

REM Create Windows batch launcher
echo @echo off> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo setlocal>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo REM Check if Java is available>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo java -version ^>nul 2^>^&1>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo if errorlevel 1 (>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo     echo ERROR: Java is not installed or not in PATH!>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo     echo Please install Java 25 or higher.>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo     pause>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo     exit /b 1>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo ^)>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo REM Run the application with arguments>> "%PACKAGE_DIR%\%APP_NAME%.bat"
echo java -Xms256m -Xmx1g -Dspring.config.additional-location=./config/ -jar "%JAR_NAME%" %%*>> "%PACKAGE_DIR%\%APP_NAME%.bat"

REM Create PowerShell launcher
echo # %APP_NAME% Launcher> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo # Usage: .\%APP_NAME%.ps1 -d 2026-03-05>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo $ErrorActionPreference = "Stop">> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo # Check if Java is available>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo try {>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     $null = Get-Command java -ErrorAction Stop>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo } catch {>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     Write-Host "ERROR: Java is not installed or not in PATH!" -ForegroundColor Red>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     Write-Host "Please install Java 25 or higher." -ForegroundColor Yellow>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     exit 1>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo }>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo # Run the application>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo $jar = "%JAR_NAME%">> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo $javaArgs = @(>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     "-Xms256m",>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     "-Xmx1g",>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo     "-jar", $jar>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo ^)>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo # Add all script arguments>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo $allArgs = $javaArgs + $args>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo.>> "%PACKAGE_DIR%\%APP_NAME%.ps1"
echo ^& java $allArgs>> "%PACKAGE_DIR%\%APP_NAME%.ps1"

echo Creating trilingual README (UTF-8)...
powershell -ExecutionPolicy Bypass -File create-readme.ps1 -OutputPath "%PACKAGE_DIR%\README.md" -AppName "%APP_NAME%" -Version "%VERSION%" -JavaRequired "Java 17 or higher"

REM Copy sample configuration
echo Copying sample configuration files...
copy /Y "src\main\resources\application.yml" "%PACKAGE_DIR%\config\application.yml.example" 2>nul
copy /Y "src\main\resources\application-local.yml" "%PACKAGE_DIR%\config\application-local.yml.example" 2>nul

echo.
echo =========================================
echo Step 4/4 - Creating ZIP distribution
echo =========================================

powershell -Command "Compress-Archive -Force -Path '%PACKAGE_DIR%' -DestinationPath '%DIST_DIR%\%APP_NAME%-v%VERSION%-win.zip'"

echo.
echo =========================================
echo Build Complete!
echo =========================================
echo.
echo Package directory: %PACKAGE_DIR%
echo ZIP file: %DIST_DIR%\%APP_NAME%-v%VERSION%-win.zip
echo.
echo Contents:
dir /B "%PACKAGE_DIR%"
echo.
echo To test the application:
echo   cd %PACKAGE_DIR%
echo   %APP_NAME%.bat -h
echo.

pause
