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

Go Style Guide (Global)
- Reference: GoCN (Google Go Style Guide translation)
  - Overview: https://gocn.github.io/styleguide/docs/01-overview/
  - Guide: https://gocn.github.io/styleguide/docs/02-guide/
  - Decisions: https://gocn.github.io/styleguide/docs/03-decisions/
  - Best Practices: https://gocn.github.io/styleguide/docs/04-best-practices/

Guiding principles (priority order)
- Clarity: make intent obvious to a reader (what it does + why it does it).
- Simplicity: prefer the simplest mechanism that meets the goal.
- Conciseness: high signal-to-noise; avoid repetition and irrelevant noise.
- Maintainability: code should be easy to change; tests should diagnose failures.
- Consistency: follow the surrounding code and repo conventions unless there is a strong reason.

Formatting
- All Go source must be `gofmt`-formatted.
- Prefer refactoring over manual line-wrapping; Go has no fixed max line length.
- If the line is truly irreducible (e.g., a long URL), allow it to stay long.

Naming
- Use MixedCaps/mixedCaps (camel case); avoid snake_case and ALL_CAPS.
- Avoid underscores in identifiers, except:
  - generated-code import paths (rename on import if needed)
  - test/benchmark/example function names in `*_test.go`
  - low-level OS/cgo interoperability (rare)
- Package names: short, all lowercase, no underscores; avoid meaningless names like `util`, `common`, `helper`.
- Receiver names: short (1-2 letters), consistent per type, derived from the type name.
- Constants: MixedCaps; name by role/meaning, not by value (avoid `MAX_*`, `kMax*`).
- Initialisms: keep consistent casing (`URL`, `ID`); prefer `appID`, not `appId`.
- Avoid `Get` prefixes for simple accessors; use `Compute`/`Fetch` when work is heavy/remote.
- Avoid repetition: don't repeat package/type/context in names when it is already implied.

Comments and docs
- Exported top-level identifiers should have doc comments; add docs for unexported APIs with non-obvious behavior.
- Doc comments are full sentences and start with the identifier name.
- Prefer comments that explain "why" over restating "what".
- Wrap long comments for readability on narrow screens (aim for ~80 cols, not a hard limit).
- Package comment sits immediately above `package ...` with no blank line.
- Use runnable examples in `*_test.go` when helpful.
- Avoid named result parameters unless they add clarity (e.g., multiple same-type returns) or are required for deferred modification.

Imports
- Group imports: standard library first, then all non-stdlib (project + vendor). Additional groups are OK if consistent.
- Rename imports only to resolve collisions or when the imported package name is unhelpful (e.g., `v1`).
- For generated proto imports: remove underscores; use `pb` / `grpc` suffix conventions when applicable.
- Blank imports (`import _`) only in `main` packages or tests that require side effects.
- Do not use dot imports (`import .`).

Errors
- Use `error` as the last return value when a function can fail.
- Error strings: no leading capitalization (unless proper noun/exported name) and no trailing punctuation.
- Do not discard errors; if it is provably safe, discard with a comment explaining why.
- Avoid in-band error signaling (-1, empty string); return `(value, ok)` or `(value, error)`.
- Prefer early returns and avoid `else` indentation for the normal path.
- Prefer structured/inspectable errors (sentinels/types) over string matching.
- Add context to errors when it helps diagnosis; avoid redundant context.
- Use `%w` only when you deliberately want callers to unwrap/inspect the underlying error.
- Be careful with error logging:
  - avoid double-logging (if you return an error, let the caller decide whether to log)
  - avoid PII in logs

Language and idioms
- Prefer composite literals over step-by-step field assignment when it is clearer.
- Use keyed fields when constructing a struct from another package; unkeyed is OK for small stable structs.
- Keep closing braces aligned; multi-line literals should end with trailing commas.
- Treat nil slices and empty slices as equivalent unless you have a strong reason; don't force callers to distinguish.
- Avoid `panic` for normal error handling; use it only for API misuse or broken internal invariants.
- Avoid subtle shadowing with `:=` in nested scopes; use `=` or rename variables when needed.

Testing
- Prefer semantic comparisons over comparing unstable strings/output.
- Prefer continuing a test with multiple checks (`t.Error*`) over failing fast (`t.Fatal*`) when useful.
- Use subtests (`t.Run`) and table-driven tests when it improves clarity and filtering.
- Make failure messages actionable: show input, got vs want, and diffs for large structures.
- Avoid depending on assertion libraries as a style decision; in this repo, tests already use `testify/assert`.

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
