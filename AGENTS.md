# Repository Guidelines

## Project Structure & Module Organization
This repository combines a Go backend with a pnpm-managed UI workspace. Core Go code lives in `main.go`, `pkg/`, and `packages/`; generated or built artifacts go to `bin/`. The main frontend app is in `packages/ui/`. Examples and reference material live in `examples/` and `docs/`. Keep new backend packages under `pkg/<domain>` and place UI-only assets beside the relevant feature in `packages/ui/src/`.

## Build, Test, and Development Commands
Use the commands that match the layer you are changing:

- `go generate ./...` rebuilds generated assets, including the packaged UI.
- `go build -o bin/nanobot .` builds the main binary.
- `.\build_windows.ps1` runs the Windows build flow end-to-end.
- `go test ./...` runs backend tests.
- `pnpm install --frozen-lockfile` installs workspace dependencies reproducibly.
- `pnpm -r run check` runs workspace checks.
- `make validate` runs formatting, vet, tests, `go mod tidy`, and UI CI checks.

## Coding Style & Naming Conventions
Go code should stay `gofmt`-clean, with small packages and explicit names. TypeScript and UI code use 2-space indentation and the existing Biome/pnpm toolchain. Follow current naming patterns: exported Go identifiers use `CamelCase`, internal helpers use `camelCase`, and scripts or workspace tasks use descriptive lowercase names. Avoid introducing new formatting conventions when editing existing files.

## Testing Guidelines
Prefer targeted regression tests close to the changed code, such as `*_test.go` for Go packages. Run the smallest relevant test first, then rerun broader validation before finishing. If you change generation or packaging behavior, verify with the real build command, not just unit tests.

## Commit & Pull Request Guidelines
Recent history favors short imperative subjects such as `Fix SSE shutdown logging in MCP HTTP client` or `Chore: Convert to slog`. Keep commits focused and descriptive. Pull requests should summarize behavior changes, list affected directories, note any required environment variables, and include screenshots or logs for UI or build-output changes.

## Security & Agent Notes
Never commit secrets; keep them in local environment files only. For agents: prefer `rg` for search, use `apply_patch` for edits, do not revert unrelated user changes, and verify claims with fresh command output before reporting completion.
