@echo off
rem procoder launcher (Windows) — see launcher.sh for the design.
setlocal
set "dir=%~dp0.."

rem PROCODER_BIN is the caller's own file and NOTHING about it is checked —
rem not its version, not its checksum. It exists for a mirror, for a bisect,
rem and for the tests, which need to point this script at a fixture.
rem
rem launcher.sh has honoured this since the launcher was written; this script
rem did not, and so every test that runs through here on Windows was judging
rem the released binary while believing it was judging a fixture.
if defined PROCODER_BIN goto viabin
goto resolved

:viabin
if not exist "%PROCODER_BIN%" goto missingbin
"%PROCODER_BIN%" %*
exit /b %ERRORLEVEL%

:missingbin
rem 127 is the shell's "command not found", borrowed on purpose. What sits on
rem the other side of this script distinguishes "the binary never ran" from
rem "the binary ran and said no" by that number, because the two otherwise read
rem identically and one of them is a gate that silently stopped judging. cmd.exe
rem has no such convention, so this script supplies it.
echo procoder: PROCODER_BIN points at "%PROCODER_BIN%" which is not there — nothing ran 1>&2
exit /b 127

:resolved
set "arch=amd64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "arch=arm64"
set "bin=%dir%\dist\windows-%arch%\procoder.exe"
if not exist "%bin%" goto missingbinary
"%bin%" %*
exit /b %ERRORLEVEL%

:missingbinary
rem A hook that cannot get its binary warns and lets the session continue; a
rem command refuses. Same split as launcher.sh, same reason: to a pre-tool-use
rem hook no stdout means "no decision", and a launcher that exits 0 having run
rem nothing is a silent green underneath every check in the tool.
echo procoder: no binary for windows/%arch% at %bin% — reinstall the plugin 1>&2
if /i "%~1"=="hook" exit /b 0
exit /b 1
