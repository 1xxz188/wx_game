# Repository Agent Guide (wx_game)

Purpose
- This is a Go backend service for a WeChat mini-game ("watermelon"), with HTTP login and WebSocket gameplay.
- Use this file as the default coding guide for automated agents in this repo.

Project Layout (high-level)
- main.go: server bootstrap, config loading, Mongo persistence, rank manager, and HTTP/WebSocket routes.
- handlers.go: HTTP login handler and app services wiring.
- websocket.go / wechat.go: WebSocket and WeChat login integration.
- rank/, role/, msg/, watermelon/: core gameplay, models, protobuf messages.
- test_client/: sample client and load test helpers.

Build / Run
- Start server (dev): `go run .`
- Build (default): `go build .`
- Debug build (Windows): `make_debug.bat` (uses `go build -gcflags="all=-N -l=0" .`)
- Linux build (Windows cross): `make_linux.bat` (sets `GOOS=linux GOARCH=amd64` then `go build .`)

Tests
- Tests require a running server and dev mode enabled in `config.yaml` (`dev_mode: true`).
- Login is rate-limited (5/min). If tests fail with 429, wait 1 minute and retry.
- Run all tests: `go test ./...`
- Run tests but skip server-dependent ones: `go test -short ./...`
- Run a single test: `go test -run TestWebSocketAuth -v ./...`
- Notes: `main_test.go` uses live HTTP/WebSocket calls to `127.0.0.1:8080` (or TLS target when enabled).

Lint / Format
- No explicit linter config found (.golangci.* not present).
- Use `gofmt` for formatting. Keep output as gofmt would produce.
- Avoid adding new lint tooling unless requested.

Protobuf Generation
- Windows helper: `msg\protos\a_make.bat` (calls `protoc.exe --go_out=.. --go_opt=paths=source_relative ./*.proto`).
- See `msg/protos/README.md` for protoc and plugin setup.

Test Client
- `test_client\Run.bat` runs `test_client.exe -count 500` (load test).
- `test_client/README.md` includes a usage example and the same server/dev-mode notes as tests.

Code Style and Conventions
- Language: Go 1.25 (see `go.mod`).
- Formatting: tabs for indentation; keep lines readable; follow existing layout.
- Imports: keep simple import blocks as seen in files; do not introduce custom grouping unless already present.
- Naming: use Go conventions (PascalCase for exported, camelCase for unexported).
- Struct tags: use JSON tags like `json:"field"` on request/response structs.
- Error handling: return early on errors; wrap with context using `fmt.Errorf` where helpful; avoid silent failures.
- Logging: use `github.com/donnie4w/go-logger/logger` for server logs and include context (IP, IDs, reason).
- HTTP handlers: prefer explicit status codes and JSON error responses via Fiber.
- WebSocket/proto: keep message framing consistent (4-byte msg ID header + protobuf payload).
- Concurrency: use explicit locks where required (see role/rank patterns); mention any goroutine lifecycle changes.

Runtime/Config Expectations
- `config.yaml` drives server settings (ports, dev_mode, logging, Mongo).
- MongoDB is required for normal startup; role/rank data loads on boot.
- WebSocket endpoint is `/ws`; login endpoint is `/api/login`.
- Security headers and rate limiting are configured in `main.go`.

Safety and Changes
- Make minimal, focused changes.
- Avoid refactors during bug fixes.
- Do not add new dependencies unless required.

Editor Rules
- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` found in this repo.
