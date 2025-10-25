# Repository Guidelines

## Project Structure & Module Organization
River is a Go CLI housed under `cmd/river` (entrypoint) and supporting packages inside `internal/` (`ai`, `editor`, `history`, `onboarding`, `statsui`). Runtime assets (prebuilt binaries) live in `river/` while npm’s launcher sits in `bin/` and install helpers in `scripts/`. Keep new features inside `internal/<feature>` and expose only through `cmd/river/main.go` to preserve the public API.

## Build, Test, and Development Commands
- `go run ./cmd/river` — run the CLI locally; pass subcommands like `stats` or `analyze`.
- `go build -o river/river ./cmd/river` — produce the binary bundled by `bin/river.js`.
- `GOEXPERIMENT=boringcrypto go build ./cmd/river` — example of forcing reproducible builds for release artifacts.
- `go test ./...` — executes every `_test.go` file (add `-run TestStats` to focus).
- `npm pack` in the repo root — smoke-test the npm wrapper and `scripts/install.js`.

## Coding Style & Naming Conventions
Use Go 1.23+ with `gofmt`/`goimports` (tabs, 120-column soft limit). Favor small, composable Bubble Tea models and keep exported names in CamelCase; unexported helpers stay lowerCamel. JSON or config structs should mirror CLI flags (`StatsOptions`, `EditorState`). For JavaScript shims, follow ESLint defaults: 2-space indent, `const`/`let`, no default exports.

## Testing Guidelines
Place `_test.go` files beside the code under test (e.g., `internal/history/history_test.go`). Prefer table-driven tests and the standard `testing` package; wrap Bubble Tea models with deterministic inputs and use golden files for multi-line renders. Aim to cover every subcommand handler plus Anthropic API adapters; add lightweight integration tests that stub API responses via env overrides. Run `go test ./... -race` before tagging a release.

## Commit & Pull Request Guidelines
Recent history uses terse, imperative subjects (`Redo stats`, `Yolo`). Keep the style but expand with a scope prefix when possible (`stats: redo chart layout`). Use conventional commits only if the change spans multiple packages. Every PR should include: summary of behavior change, `go test ./...` results, screenshots/gifs for `statsui` updates, and references to the related issue or discussion.

## Security & Configuration Tips
`internal/ai` reads `ANTHROPIC_API_KEY` from the environment or onboarding prompts—never commit keys or the generated `.river.env`. Validate inputs before sending to Anthropic, and prefer `os.LookupEnv` to avoid accidental defaulting. When testing installer flows, scrub binaries from `bin/` and rerun `npm install -g .` so installers fetch freshly built artifacts.
