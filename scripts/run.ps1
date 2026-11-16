# app CLI Runner
# Usage: .\run.ps1 -d 2026-03-03

param(
    [Parameter(ValueFromRemainingArguments=$true)]
    $Arguments
)

$argsString = $Arguments -join ' '
mvn spring-boot:run "-Dspring-boot.run.arguments=$argsString" "-Dspring-boot.run.profiles=local"
