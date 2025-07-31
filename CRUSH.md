# River Go Project Commands & Conventions

## Build & Test Commands
- Build: `go build ./...`
- Test all: `go test ./...`
- Test single: `go test -run TestName ./path/to/package`
- Test verbose: `go test -v ./...`
- Lint: `golangci-lint run`
- Format: `go fmt ./...`
- Tidy deps: `go mod tidy`

## Code Style Guidelines
- Use `gofmt` for formatting (no tabs/spaces debates)
- Import grouping: stdlib, external deps, internal packages (separated by blank lines)
- Error handling: return early, wrap errors with context using `fmt.Errorf("context: %w", err)`
- Naming: use camelCase for vars/funcs, PascalCase for exported types
- Interfaces: small, focused interfaces (1-3 methods preferred)
- Comments: exported functions need godoc comments starting with function name

## Project Structure
- `/cmd` - Main applications
- `/internal` - Private application code
- `/pkg` - Public libraries
- `/api` - API definitions
- `/test` - Additional test data

## Testing Conventions
- Test files alongside code: `foo.go` → `foo_test.go`
- Use table-driven tests with subtests: `t.Run(name, func(t *testing.T) {...})`
- Mock interfaces, not structs
- Test package naming: same package for white-box, `package_test` for black-box