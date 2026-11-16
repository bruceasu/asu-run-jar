@echo off
REM app CLI Runner
REM Usage: run.bat -d 2026-03-03

mvn spring-boot:run "-Dspring-boot.run.arguments=%*" "-Dspring-boot.run.profiles=local"
