@echo off
setlocal
rem Forward all platform, architecture and UPX options to the shared build script.
rem Usage: build.bat [-l] [-m] [-w] [-amd] [-arm] [-a] [-x]
where sh >nul 2>&1
if errorlevel 1 (
    echo Error: Install Git for Windows and make sh available on PATH. >&2
    exit /b 1
)
pushd "%~dp0"
if errorlevel 1 exit /b 1
sh build.sh %*
set "BUILD_EXIT=%ERRORLEVEL%"
popd
exit /b %BUILD_EXIT%
