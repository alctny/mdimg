@echo off
setlocal
where go >nul 2>&1
if errorlevel 1 (
    echo Error: Go 1.23 or newer must be installed and available on PATH. >&2
    exit /b 1
)
pushd "%~dp0"
if errorlevel 1 exit /b 1
go run ./scripts/build.go
set "BUILD_EXIT=%ERRORLEVEL%"
popd
exit /b %BUILD_EXIT%
