# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

### Backend (Go)
```bash
make run                    # Build and run with hot reload (uses bra)
make build-backend          # Build backend only
make build-go               # Build all Go binaries (includes code gen)
go run build.go build       # Alternative build command
```

### Frontend (TypeScript/React)
```bash
yarn install --immutable    # Install dependencies
yarn start                  # Dev server with hot reload
yarn build                  # Production build
```

### Full Build
```bash
make build                  # Build both backend and frontend
make build-docker-full      # Build Docker image (tagged grafana/grafana:dev)
```

## Testing

### Backend Tests
```bash
go test -v ./pkg/...                           # Run all backend tests
go test -short -covermode=atomic ./pkg/...     # Unit tests only
go test -run Integration ./pkg/...             # Integration tests only
go test -v ./pkg/services/dashboardsnapshots/... # Test specific package
```

### Frontend Tests
```bash
yarn test                   # Interactive test mode
yarn test:ci                # CI mode with coverage
```

### E2E Tests
```bash
yarn e2e                    # Run Cypress e2e tests
yarn e2e:debug              # Run with browser visible
```

## Linting

```bash
make lint-go                # Go linting (golangci-lint)
yarn lint                   # Frontend linting (eslint + stylelint)
yarn lint:fix               # Auto-fix frontend lint issues
```

## Code Generation

```bash
make gen-go                 # Generate Wire DI and CUE code
make gen-cue                # Generate code from .cue files only
```

## Architecture Overview

### Backend (Go)
Located in `/pkg/`:
- `/pkg/api` - HTTP handlers and routing
- `/pkg/cmd` - Main binaries: grafana-server, grafana-cli
- `/pkg/services` - Domain services with DI (the core business logic)
- `/pkg/infra` - Infrastructure packages (logging, database, etc.)
- `/pkg/tsdb` - Data source backend implementations
- `/pkg/models` - Domain models
- `/pkg/setting` - Configuration via setting.Cfg struct

Key patterns:
- Use Wire for dependency injection
- Services communicate via DI, not global state
- Pass `context.Context` through all layers
- Integration tests: name functions `TestIntegrationXxx` with `if testing.Short() { t.Skip() }`

### Frontend (TypeScript/React)
Located in `/public/app/`:
- `/public/app/features` - Feature modules (dashboards, alerting, explore, etc.)
- `/public/app/core` - Core utilities and components
- `/public/app/plugins` - Built-in datasources and panels
- `/public/app/store` - Redux store configuration

Packages in `/packages/`:
- `@grafana/ui` - Reusable UI components
- `@grafana/data` - Data utilities and types
- `@grafana/runtime` - Runtime services

## Development Environment

### Dev Services
```bash
make devenv sources=postgres,influxdb   # Start data sources via Docker
make devenv-down                        # Stop dev services
```

### Configuration
- Default config: `conf/defaults.ini`
- Custom config: `conf/custom.ini` (create this file for local overrides)
- Enable dev mode: add `app_mode = development` to custom.ini

### Default Login
- Username: `admin`
- Password: `admin`

## Backend Testing Conventions

- Use `testing` stdlib with `testify` for assertions
- Use `require.*` for assertions that should halt the test
- Use `assert.*` for soft checks
- Use `t.Run()` for sub-tests
- Use `t.Cleanup()` instead of defer for cleanup
- Use mockery for generating mocks: `mockery --name InterfaceName --inpackage`
