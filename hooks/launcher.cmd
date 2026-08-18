@echo off
rem procoder launcher (Windows) — see launcher.sh for the design.
setlocal
set "dir=%~dp0.."
set "arch=amd64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "arch=arm64"
set "bin=%dir%\dist\windows-%arch%\procoder.exe"
if not exist "%bin%" (
  echo procoder: no binary for windows/%arch% at %bin% — reinstall the plugin 1>&2
  exit /b 1
)
"%bin%" %*
