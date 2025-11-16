@echo off
protoc.exe --go_out=.. --go_opt=paths=source_relative ./*.proto

if %ERRORLEVEL% EQU 0 (
    if not exist "..\..\..\wx_game_tools\protobuf\proto" (
        echo Warning: Target directory does not exist, skipping copy operation
    ) else (
        for %%f in (*.proto) do (
            copy /Y "%%f" "..\..\..\wx_game_tools\protobuf\proto\" >nul 2>&1
            echo Copying: %%f
        )
    )
) else (
    echo Export failed, skipping copy operation
)

pause