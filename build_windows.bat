@echo off
setlocal
cd /d "%~dp0"
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go test ./...
if errorlevel 1 exit /b 1
go build -trimpath -ldflags="-s -w -H=windowsgui" -o "心阅小说_v4.1.3_全功能回归修复版_Windows_x64.exe" .
if errorlevel 1 exit /b 1
echo Build complete.
pause
