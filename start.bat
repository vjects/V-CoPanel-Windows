@echo off
chcp 65001 >nul 2>&1
title V-CoPanel Windows

if not exist "workspace\core" mkdir "workspace\core"

tasklist /FI "IMAGENAME eq bridge.exe" 2>NUL | find /I /N "bridge.exe">NUL
if "%ERRORLEVEL%"=="0" (
    echo [PID - start.bat] Bridge Engine is already running. Graceful exit.
    exit /b 0
)

if exist "workspace\core-assets" (
    xcopy /E /I /Y "workspace\core-assets\*" "workspace\core\"
    rd /s /q "workspace\core-assets"
)
if not exist "workspace\core\vcredist_installed.flag" if exist "pc-assets\vcredist\VC_redist.x64.exe" (
    echo [PID - start.bat] Installing required Visual C++ Redistributable...
    start /wait "" "pc-assets\vcredist\VC_redist.x64.exe" /install /quiet /norestart
    echo done > "workspace\core\vcredist_installed.flag"
)

if exist "bridge.exe" goto START_BRIDGE
if exist "workspace\core\go\bin\go.exe" goto BUILD_ENGINE
if exist "workspace\core\go\go\bin\go.exe" goto BUILD_ENGINE
if exist "workspace\go\bin\go.exe" goto BUILD_ENGINE
if exist "workspace\go\go\bin\go.exe" goto BUILD_ENGINE

set GO_ZIP=
for %%f in ("pc-assets\go\*.zip") do set "GO_ZIP=%%f"
if not defined GO_ZIP for %%f in ("pc-assets\go*.zip") do set "GO_ZIP=%%f"
if not defined GO_ZIP goto NO_GO_ZIP

if not exist "workspace\core" mkdir "workspace\core"

echo [PID - start.bat] Unpacking portable Go runtime from pc-assets... (Please wait)
if exist "%ProgramFiles%\7-Zip\7z.exe" goto UNPACK_7Z
if exist "%ProgramFiles%\WinRAR\winrar.exe" goto UNPACK_WINRAR
where tar.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 goto UNPACK_TAR

powershell -NoProfile -Command "Expand-Archive -Path '%GO_ZIP%' -DestinationPath 'workspace\core' -Force"
goto UNPACK_DONE

:UNPACK_7Z
"%ProgramFiles%\7-Zip\7z.exe" x "%GO_ZIP%" -o"workspace\core" -y
goto UNPACK_DONE

:UNPACK_WINRAR
"%ProgramFiles%\WinRAR\winrar.exe" x -ibck -y "%GO_ZIP%" "workspace\core\"
goto UNPACK_DONE

:UNPACK_TAR
tar.exe -xf "%GO_ZIP%" -C "workspace\core"
goto UNPACK_DONE

:NO_GO_ZIP
echo [PID - start.bat] Fatal: Go portable archive was not found in pc-assets!
pause
exit /b 1

:UNPACK_DONE

:BUILD_ENGINE
echo [PID - start.bat] Compiling V-CoPanel Bridge Engine...
set GOROOT=
if exist "%CD%\workspace\core\go\go\bin\go.exe" set "PATH=%CD%\workspace\core\go\go\bin;%PATH%"
if not exist "%CD%\workspace\core\go\go\bin\go.exe" if exist "%CD%\workspace\core\go\bin\go.exe" set "PATH=%CD%\workspace\core\go\bin;%PATH%"
if not exist "%CD%\workspace\core\go\bin\go.exe" if exist "%CD%\workspace\go\go\bin\go.exe" set "PATH=%CD%\workspace\go\go\bin;%PATH%"
if not exist "%CD%\workspace\core\go\bin\go.exe" if not exist "%CD%\workspace\go\go\bin\go.exe" if exist "%CD%\workspace\go\bin\go.exe" set "PATH=%CD%\workspace\go\bin;%PATH%"

go build -ldflags "-s -w" -o bridge.exe main.go
if %ERRORLEVEL% NEQ 0 goto BUILD_ERROR
goto START_BRIDGE

:BUILD_ERROR
echo [PID - start.bat] Compilation failed!
pause
exit /b 1

:START_BRIDGE
echo [PID - start.bat] Launching V-CoPanel Bridge Engine...
bridge.exe

cmd /k
