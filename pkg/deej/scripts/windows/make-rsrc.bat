@ECHO OFF

REM Rewrites pkg\deej\cmd\rsrc_windows_amd64.syso (icon, manifest and version info for deej.exe)
REM from winres\winres.json, using go-winres. Run it from the repository root.

ECHO Generating Windows resources...
go generate ./pkg/deej/cmd
IF %ERRORLEVEL% NEQ 0 EXIT /B 1
ECHO Done.
