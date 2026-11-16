@echo off
setlocal enabledelayedexpansion
set APP_NAME=app
set MAIN_CLASS=org.springframework.boot.loader.launch.JarLauncher

set BUILD_DIR=build
set DEPS_DIR=%BUILD_DIR%\deps
set APP_DIR=%BUILD_DIR%\app
set RUNTIME_DIR=%BUILD_DIR%\runtime
set DIST_DIR=dist

echo Cleaning old builds...
rmdir /s /q %BUILD_DIR% 2>nul
rmdir /s /q %DIST_DIR% 2>nul

mkdir %BUILD_DIR%
mkdir %DEPS_DIR%
mkdir %APP_DIR%

call setup-jdk25.bat
echo -----------------------------------------
echo Step 1 - Build Spring Boot jar
echo -----------------------------------------

call mvnw.cmd -q clean package -DskipTests

if errorlevel 1 (
    echo ERROR: Maven build failed!
    pause
    exit /b 1
)

for %%f in (target\*.jar) do (
    set JAR=%%f
    set JAR_NAME=%%~nxf
)

if not defined JAR_NAME (
    echo ERROR: No JAR file found in target directory!
    pause
    exit /b 1
)

echo Found jar: %JAR%
echo JAR name: %JAR_NAME%

echo -----------------------------------------
echo Step 2 - Prepare app directory
echo -----------------------------------------

echo Copying JAR to app directory...
copy /Y "%JAR%" "%APP_DIR%\%JAR_NAME%"

echo -----------------------------------------
echo Step 3 - Copy runtime dependencies
echo -----------------------------------------

call mvn.cmd dependency:copy-dependencies ^
 -DincludeScope=runtime ^
 -DoutputDirectory=%DEPS_DIR%

echo -----------------------------------------
echo Step 4 - Build classpath for jdeps
echo -----------------------------------------

set CP=

for %%f in (%DEPS_DIR%\*.jar) do (
 set CP=!CP!;%%f
)

set CP=%CP:~1%

echo -----------------------------------------
echo Step 5 - Detect required Java modules
echo -----------------------------------------

for /f %%i in ('jdeps ^
 --ignore-missing-deps ^
 --multi-release 25 ^
 --recursive ^
 --print-module-deps ^
 --class-path "%CP%" ^
 %JAR%') do set MODULES=%%i

echo Detected modules:
echo %MODULES%

echo -----------------------------------------
echo Step 6 - Build optimized runtime
echo -----------------------------------------

jlink ^
 --add-modules %MODULES%,jdk.unsupported,java.naming ^
 --strip-debug ^
 --compress=2 ^
 --no-header-files ^
 --no-man-pages ^
 --output %RUNTIME_DIR%

if errorlevel 1 (
    echo ERROR: jlink failed!
    pause
    exit /b 1
)

echo -----------------------------------------
echo Step 7 - Package application
echo -----------------------------------------

jpackage ^
 --type app-image ^
 --name %APP_NAME% ^
 --input %APP_DIR% ^
 --main-jar %JAR_NAME% ^
 --main-class %MAIN_CLASS% ^
 --runtime-image %RUNTIME_DIR% ^
 --dest %DIST_DIR% ^
 --java-options "-Xms256m" ^
 --java-options "-Xmx1g" ^
 --java-options "-DJASYPT_ENCRYPTOR_PASSWORD=fxtf-daily-stat-secret" ^
 --win-console

if errorlevel 1 (
    echo ERROR: jpackage failed!
    pause
    exit /b 1
)

echo -----------------------------------------
echo Step 7.5 - Copy documentation and config
echo -----------------------------------------

REM Copy USAGE.md to distribution
echo Copying USAGE.md...
copy /Y "USAGE.md" "%DIST_DIR%\%APP_NAME%\USAGE.md"

REM Copy sample configuration files to app directory (where JAR is located)
echo Copying sample configuration files to app directory...
copy /Y "src\main\resources\application.yml" "%DIST_DIR%\%APP_NAME%\app\application.yml.example" 2>nul
copy /Y "src\main\resources\application-local.yml" "%DIST_DIR%\%APP_NAME%\app\application-local.yml.example" 2>nul

REM Create helper launchers for runtime JVM options and passthrough app args
echo Creating run.bat and run.ps1...
(
  echo @echo off
  echo setlocal
  echo set "SCRIPT_DIR=%%~dp0"
  echo if not "%%APP_JAVA_OPTS%%"=="" set "JDK_JAVA_OPTIONS=%%APP_JAVA_OPTS%% %%JDK_JAVA_OPTIONS%%"
  echo "%%SCRIPT_DIR%%bin\%APP_NAME%.exe" %%*
) > "%DIST_DIR%\%APP_NAME%\run.bat"

(
  echo param^( [Parameter^(ValueFromRemainingArguments = $true^)] [string[]] $Args ^)
  echo $ErrorActionPreference = "Stop"
  echo $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
  echo if ^($env:APP_JAVA_OPTS^) { $env:JDK_JAVA_OPTIONS = "$($env:APP_JAVA_OPTS^) $($env:JDK_JAVA_OPTIONS^)".Trim^(^) }
  echo ^& "$scriptDir\bin\%APP_NAME%.exe" @Args
  echo exit $LASTEXITCODE
) > "%DIST_DIR%\%APP_NAME%\run.ps1"

echo.
echo NOTE: Configuration files should be placed in the app/ directory.
echo NOTE: For runtime JVM options, set APP_JAVA_OPTS or JDK_JAVA_OPTIONS.
echo.

echo -----------------------------------------
echo Step 8 - Create zip distribution
echo -----------------------------------------

powershell -Command "Compress-Archive -Force -Path %DIST_DIR%\%APP_NAME% -DestinationPath %DIST_DIR%\%APP_NAME%-win.zip"

echo Build complete.
echo Distribution: %DIST_DIR%\%APP_NAME%-win.zip

pause
