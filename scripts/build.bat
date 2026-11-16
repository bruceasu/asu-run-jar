@echo off
setlocal
set CWD=%CD%
cd /d %~dp0..

if /i "%~1"=="all" (
	call go-build -project ajava
	call cpFile.bat bin\ajava-windows-amd64.exe c:\green\ajava.exe
) else (
	rem windows, linux, darwin
	set GOOS=windows

	rem amd64, 386
	set GOARCH=amd64

	set CGO_ENABLED=0

	go build -ldflags="-s -w" -v -o ajava.exe ./cmd/ajava

	call cpFile.bat ajava.exe c:\green\ajava.exe
)

:: GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ajava.exe ./cmd/ajava
:: GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ajava-linux ./cmd/ajava
:: gbuild
cd /d %CWD%
endlocal

pause
