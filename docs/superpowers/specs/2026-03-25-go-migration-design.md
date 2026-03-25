# Go Migration Design

Migrate claudebox from Bash to Go for type safety and extensibility.

## Context

claudebox is a ~277-line Bash CLI (5 modules + helpers) that runs Claude Code inside sandboxed Docker containers. It has ~750 lines of BATS unit tests and ~150 lines of BATS integration tests. The project is well-structured but Bash is becoming a liability for error handling and future feature work.

## Approach

Clean rewrite in idiomatic Go. The current Bash serves as the behavioral spec. No gradual transition — write Go, verify with integration tests, delete Bash.

## CLI Interface

The interface stays identical:

- `claudebox <template>` — create and run a sandbox from a template
- `claudebox resume` — resume an existing sandbox
- `claudebox ls` — list sandboxes for the current workspace
- `claudebox rm <name|--all>` — remove sandboxes

## Dependencies

- **cobra** for CLI framework (subcommand routing, flags, help)
- **Go stdlib** for everything else (`os/exec`, `path/filepath`, `fmt`, `strings`, `os`, `testing`)

No viper, no testify, no other third-party libraries.

## Project Layout

```
claudebox/
├── cmd/
│   └── claudebox/
│       └── main.go              # Entry point, cobra root command
├── internal/
│   ├── commands/
│   │   ├── create.go            # claudebox <template>
│   │   ├── resume.go            # claudebox resume
│   │   ├── ls.go                # claudebox ls
│   │   └── rm.go                # claudebox rm
│   ├── docker/
│   │   └── docker.go            # Docker CLI wrapper
│   ├── credentials/
│   │   └── keychain.go          # macOS Keychain reads
│   ├── environment/
│   │   └── environment.go       # Sandbox env setup (proxy, JVM config)
│   └── sandbox/
│       └── sandbox.go           # Core sandbox lifecycle operations
├── templates/
│   └── jvm/
│       ├── Dockerfile
│       └── allowed-hosts.txt
├── tests/
│   └── integration/
│       ├── create_test.go
│       ├── network_test.go
│       ├── filesystem_test.go
│       └── cleanup_test.go
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

- `internal/` prevents external imports — everything is private to this binary.
- Unit tests live alongside source files (Go convention): `docker_test.go`, `sandbox_test.go`, etc.
- Integration tests live in `tests/integration/` with a build tag.
- Templates stay as-is (Docker files, not Go code).

## Package Responsibilities

### `internal/docker/`

Thin wrapper over `docker` CLI calls via `os/exec`. Exposes a struct and an interface:

```go
type Docker interface {
    Build(tag string, contextDir string) error
    SandboxCreate(name string, opts SandboxCreateOpts) error
    SandboxRun(name string, args ...string) error
    SandboxExec(name string, cmd string) (string, error)
    SandboxLs(filter string) ([]SandboxInfo, error)
    SandboxRm(name string) error
}
```

The interface is the mock boundary for unit tests.

### `internal/sandbox/`

Core business logic for sandbox lifecycle. Takes a `Docker` interface. Operations:

- Build template image
- Create sandbox with workspace mount and Claude config symlinks
- Copy workspace to container-local path (avoids VirtioFS latency)
- Create isolated git branch (`sandbox/<name>`)
- Apply network policy from `allowed-hosts.txt` (deny-by-default + whitelist)
- Wrap claude binary to cd to workspace on launch

### `internal/credentials/`

Reads credentials from macOS Keychain via `security find-generic-password`, base64 encodes, injects into sandbox via docker exec.

### `internal/environment/`

Sets up sandbox environment: exports `GITHUB_USERNAME`, configures JVM HTTP/HTTPS proxy settings, imports proxy CA certificate into Java truststore. Injects via docker exec.

### `internal/commands/`

Thin cobra command constructors. Each command: parses flags, validates args, calls into sandbox/credentials/environment, handles user-facing output and errors.

## Command Flows

### `claudebox <template>` (create)

1. Validate template exists (has `Dockerfile` in `templates/<name>/`)
2. Build Docker image: `docker build -t claudebox-<template> templates/<template>/`
3. Determine workspace (current git repo root) and sandbox name (`claudebox-<workspace>-<template>-<timestamp>`)
4. Create sandbox with workspace mount and Claude config symlinks
5. Copy workspace files to container-local path
6. Create isolated git branch (`sandbox/<name>`)
7. If `allowed-hosts.txt` exists, apply network policy
8. Wrap claude binary to cd to workspace on launch
9. Refresh credentials and set up environment
10. Run sandbox with `--dangerously-skip-permissions`

### `claudebox resume`

1. List sandboxes matching current workspace
2. If none: error. If one: prompt to confirm. If multiple: interactive picker.
3. Refresh credentials and environment
4. Resume sandbox, passing any extra args

### `claudebox ls`

1. Run `docker sandbox ls`, filter to current workspace prefix

### `claudebox rm <name|--all>`

1. If `--all`: find and remove all sandboxes for current workspace
2. If name: validate it exists, remove it
3. Report count removed

All commands return structured errors. Cobra handles usage/help.

## Testing Strategy

### Unit tests

Live alongside source files in `internal/`. Mock boundary is the `Docker` interface.

- `sandbox_test.go`: verifies `Create()` calls docker methods in correct order with correct args
- `docker_test.go`: verifies CLI command construction
- `credentials_test.go`, `environment_test.go`: mock docker exec for injection logic
- Command-level tests: flag parsing, arg validation, error output

### Integration tests

Build tag `//go:build integration` in `tests/integration/`. Run with `go test -tags integration ./tests/integration/`.

- Hit real Docker sandbox CLI
- Same coverage as current BATS suite: image building, sandbox creation, network policy enforcement, filesystem layout, cleanup
- Test helpers: `buildTemplateImage()`, `createTestSandbox()`, `cleanupTestSandbox()`, etc.
- Skipped if Docker sandbox isn't available (check in `TestMain`)

### Makefile targets

```makefile
test:              go test ./...
test-unit:         go test ./...
test-integration:  go test -tags integration ./tests/integration/
test-all:          go test ./... && go test -tags integration ./tests/integration/
```

## Bash Removal

The Go binary fully replaces all Bash source. Deleted at the end of migration:

- `claudebox` (bash entry point)
- `src/` (entire directory)
- `tests/unit/`, `tests/test_helper/`, `tests/setup_test_deps.sh` (BATS infrastructure)

Retained:
- `templates/` (Docker files)
- `docs/` (design specs and plans)
- `README.md` (updated for Go build/install instructions)
