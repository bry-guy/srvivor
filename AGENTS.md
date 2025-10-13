# Agent Guidelines for srvivor

## Build/Lint/Test Commands
- **Build**: `make build` or `go build -o bin/srvivor .`
- **Test all**: `make test` (runs `SRVVR_LOG_LEVEL=DEBUG go test -v ./internal/*`)
- **Test single**: `go test -run TestName ./internal/scorer` (replace TestName with specific test)
- **Lint**: `golangci-lint run` (enabled: govet, errcheck, staticcheck, unused, gocritic, stylecheck, gosec, gofmt, goimports)

## Code Style Guidelines
- **Go version**: 1.24.0
- **Imports**: Group standard library, third-party, then local packages
- **Naming**: PascalCase for exported types/functions, camelCase for unexported
- **Error handling**: Return errors, avoid panics
- **Logging**: Use slog package
- **Testing**: Use testify/assert for assertions
- **Avoid**: Unnecessary destructuring, else statements, try/catch, any types, let statements
- **Variables**: Prefer single word names where possible
- **Comments**: Add for complex functions explaining purpose
- **Formatting**: Follow gofmt/goimports standards