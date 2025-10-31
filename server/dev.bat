@echo off
setlocal

REM Configuration
set TARGET_TEST=./...
set REEARTH_DB=mongodb://localhost
set DOCKER_COMPOSE=docker compose -f ../docker-compose.yml
set DOCKER_COMPOSE_DEV=docker compose -f ../docker-compose.dev.yml

REM Parse command
if "%1"=="" goto help
if "%1"=="help" goto help
if "%1"=="dev-install" goto dev-install
if "%1"=="dev" goto dev
if "%1"=="run" goto run
if "%1"=="down" goto down
if "%1"=="run-app" goto run-app
if "%1"=="run-cerbos" goto run-cerbos
if "%1"=="run-migration" goto run-migration
if "%1"=="test" goto test
if "%1"=="gql" goto gql
if "%1"=="generate" goto generate
echo Unknown target: %1
goto help

:help
echo Usage:
echo   dev.bat ^<target^>
echo.
echo Targets:
echo   dev-install       Install tools for dev (air, mockgen)
echo   dev               Run the application with hot reloading
echo   run               Run reearth-cerbos and reearth-accounts via docker-compose
echo   down              Stop and remove containers started by run
echo   run-app           Run the application
echo   run-cerbos        Run the Cerbos server
echo   run-migration     Run database migrations
echo   test              Run tests
echo   gql               Generate GraphQL code including dataloader
echo   generate          Run go generate
goto end

:dev-install
echo Checking and installing development tools...
where air >nul 2>&1
if errorlevel 1 (
    echo Installing air...
    go install github.com/air-verse/air@v1.61.5
) else (
    echo air is already installed.
)
where mockgen >nul 2>&1
if errorlevel 1 (
    echo Installing mockgen...
    go install go.uber.org/mock/mockgen@v0.5.0
) else (
    echo mockgen is already installed.
)
goto end

:dev
echo Starting development server with hot reload...
call :dev-install
air
goto end

:run
echo Starting reearth-accounts-dev with Docker Compose...
%DOCKER_COMPOSE_DEV% up reearth-accounts-dev
goto end

:down
echo Stopping Docker Compose services...
%DOCKER_COMPOSE_DEV% down
goto end

:run-app
echo Running reearth-accounts...
go run ./cmd/reearth-accounts
goto end

:run-cerbos
echo Starting Cerbos server...
%DOCKER_COMPOSE% up -d reearth-cerbos
goto end

:run-migration
echo Running database migrations...
set RUN_MIGRATION=true
go run ./cmd/reearth-accounts
goto end

:test
echo Running tests...
set REEARTH_DB=%REEARTH_DB%
go test %TARGET_TEST%
goto end

:gql
echo Generating GraphQL code...
go generate ./internal/adapter/gql
goto end

:generate
echo Running go generate...
call :dev-install
go generate ./...
goto end

:end
endlocal
