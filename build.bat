@echo off
echo === Threadix Build Script ===
echo Embedding icon and building Windows GUI executable...
setlocal enabledelayedexpansion

set "ROOT=%~dp0"
set "ICON=%ROOT%resources\app.ico"
set "SYSO=%ROOT%threadix.syso"

if not exist "%ICON%" (
    echo ERROR: icon file not found: %ICON%
    exit /b 1
)

if not exist "%SYSO%" (
    set "GOBIN="
    set "GOPATH="
    for /f "usebackq delims=" %%G in (`go env GOBIN`) do set "GOBIN=%%G"
    for /f "usebackq delims=" %%G in (`go env GOPATH`) do set "GOPATH=%%G"
    if "!GOBIN!"=="" set "GOBIN=!GOPATH!\bin"
    if "!GOBIN!"=="!GOPATH!" set "GOBIN=!GOPATH!\bin"
    if exist "!GOPATH!\bin\rsrc.exe" set "GOBIN=!GOPATH!\bin"

    set "RSRCPATH="
    if exist "!GOBIN!\rsrc.exe" set "RSRCPATH=!GOBIN!\rsrc.exe"
    if "!RSRCPATH!"=="" (
        echo Installing rsrc tool...
        go install github.com/akavel/rsrc@latest
        if errorlevel 1 (
            echo ERROR: failed to install rsrc
            pause
            exit /b 1
        )
        if exist "!GOBIN!\rsrc.exe" set "RSRCPATH=!GOBIN!\rsrc.exe"
        if "!RSRCPATH!"=="" if exist "!GOPATH!\bin\rsrc.exe" set "RSRCPATH=!GOPATH!\bin\rsrc.exe"
    )

    if "!RSRCPATH!"=="" (
        echo ERROR: rsrc.exe not found
        pause
        exit /b 1
    )

    echo Generating Windows resource from icon...
    "!RSRCPATH!" -ico "%ICON%" -o "%SYSO%"
    if errorlevel 1 (
        echo ERROR: failed to generate resource file
        pause
        exit /b 1
    )
)

cd /d "%ROOT%"
go build -buildvcs=false -ldflags "-H=windowsgui" -o threadix.exe .
if %ERRORLEVEL% EQU 0 (
    echo Build successful: threadix.exe
) else (
    echo Build failed!
    pause
)
endlocal
