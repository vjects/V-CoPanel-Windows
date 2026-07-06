@echo off
REM ============================================================================
REM V-CoPanel Windows - Smart Desktop Launcher Script
REM ============================================================================

REM Check if bridge.exe is already running in background
tasklist /FI "IMAGENAME eq bridge.exe" 2>NUL | find /I /N "bridge.exe">NUL

if "%ERRORLEVEL%"=="0" (
    REM Server is active -> open the panel in default system browser
    start "" "http://localhost:8880"
) else (
    REM Server is not active -> execute main start.bat to boot infrastructure
    call "%~dp0..\start.bat"
)
