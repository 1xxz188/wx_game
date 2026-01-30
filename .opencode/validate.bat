@echo off
setlocal enabledelayedexpansion

REM wx_game Code Quality Check Script (Windows)

echo ========================================
echo wx_game Code Quality Check
echo ========================================

REM 1. Format Check
echo [CHECK] Checking code format...
gofmt -l . > nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Code format correct
) else (
    echo [FAIL] Code format issues found. Run: gofmt -w .
    exit /b 1
)

REM 2. go vet
echo [CHECK] Running go vet...
go vet ./...
if %errorlevel% equ 0 (
    echo [OK] go vet passed
) else (
    echo [FAIL] go vet failed
    exit /b 1
)

REM 3. golangci-lint (optional)
where golangci-lint >nul 2>&1
if %errorlevel% equ 0 (
    echo [CHECK] Running golangci-lint...
    golangci-lint run --timeout 5m
    if !errorlevel! equ 0 (
        echo [OK] golangci-lint passed
    ) else (
        echo [FAIL] golangci-lint failed
        exit /b 1
    )
) else (
    echo [WARN] golangci-lint not installed, skipping
)

REM 4. Unit Tests
echo [CHECK] Running unit tests...
go test ./... -v -coverprofile=coverage.out
if %errorlevel% equ 0 (
    echo [OK] Unit tests passed
    go tool cover -func=coverage.out | findstr "total"
) else (
    echo [FAIL] Unit tests failed
    exit /b 1
)

REM 5. Race Detection
echo [CHECK] Running race detection...
go test -race ./... -timeout 30s
if %errorlevel% equ 0 (
    echo [OK] Race detection passed
) else (
    echo [FAIL] Data race detected
    exit /b 1
)

REM 6. Build Check
echo [CHECK] Build verification...
go build -o wx_game.exe main.go
if %errorlevel% equ 0 (
    echo [OK] Build successful
    del wx_game.exe
) else (
    echo [FAIL] Build failed
    exit /b 1
)

REM 7. Dependency Check
echo [CHECK] Checking dependencies...
go mod verify
if %errorlevel% equ 0 (
    echo [OK] Dependencies verified
) else (
    echo [FAIL] Dependencies verification failed
    exit /b 1
)

echo.
echo ========================================
echo [SUCCESS] All checks passed!
echo ========================================
