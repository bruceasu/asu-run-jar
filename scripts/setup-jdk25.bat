@echo off
REM Set JDK 25 as JAVA_HOME for this build session
set JAVA_HOME=C:\green\jdk25.0.1_8
set PATH=%JAVA_HOME%\bin;%PATH%

echo Using JDK: %JAVA_HOME%
echo.
echo Java Version:
"%JAVA_HOME%\bin\java.exe" -version
