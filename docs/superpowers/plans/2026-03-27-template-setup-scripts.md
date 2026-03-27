# Per-Template Setup Scripts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move JVM-specific environment setup out of generic Go code and into a per-template `setup.sh` convention, running once at create time.

**Architecture:** Each template can optionally include a `setup.sh` that the sandbox manager reads from the template directory and executes inside the container at create time. Same discovery pattern as `allowed-hosts.txt`. The `internal/environment` package is deleted; GITHUB_USERNAME moves inline to `create.go`.

**Tech Stack:** Go, shell scripts, Docker

---

### Task 1: Create `templates/jvm/setup.sh`

**Files:**
- Create: `templates/jvm/setup.sh`

- [ ] **Step 1: Create the setup script**

Extract the JVM proxy and CA cert logic from `internal/environment/environment.go:27-36` into a standalone shell script:

```bash
#!/bin/sh
# JVM proxy and CA cert configuration.
# Runs inside the sandbox at create time.

# Configure JVM proxy if HTTPS_PROXY is set (injected by Docker Desktop)
if [ -n "$HTTPS_PROXY" ]; then
  PROXY_HOST=$(echo "$HTTPS_PROXY" | sed -E "s|https?://||;s|:.*||")
  PROXY_PORT=$(echo "$HTTPS_PROXY" | sed -E "s|.*:([0-9]+).*|\1|")
  echo "export JAVA_TOOL_OPTIONS=\"-Dhttp.proxyHost=${PROXY_HOST} -Dhttp.proxyPort=${PROXY_PORT} -Dhttps.proxyHost=${PROXY_HOST} -Dhttps.proxyPort=${PROXY_PORT}\"" >> /etc/sandbox-persistent.sh
fi

# Import proxy CA cert into Java truststore
JAVA_HOME=$(java -XshowSettings:properties 2>&1 | grep "java.home" | awk '{print $3}')
PROXY_CERT=$(find /usr/local/share/ca-certificates -name "*.crt" 2>/dev/null | head -1)
if [ -n "$PROXY_CERT" ] && [ -n "$JAVA_HOME" ]; then
  sudo keytool -import -trustcacerts -cacerts -storepass changeit -noprompt -alias proxy-ca -file "$PROXY_CERT" 2>/dev/null || true
fi
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x templates/jvm/setup.sh`

- [ ] **Step 3: Commit**

```bash
git add templates/jvm/setup.sh
git commit -m "feat: add JVM template setup script

Extract JVM proxy and CA cert configuration from environment.go
into templates/jvm/setup.sh for per-template setup convention."
```

---

### Task 2: Add `RunSetupScript()` to sandbox manager

**Files:**
- Modify: `internal/sandbox/sandbox.go` (add method after `Create`)
- Modify: `internal/sandbox/sandbox_test.go` (add tests)

- [ ] **Step 1: Write failing tests**

Add to `internal/sandbox/sandbox_test.go`:

```go
func TestRunSetupScript(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "jvm"), 0o755)
	os.WriteFile(filepath.Join(dir, "jvm", "setup.sh"), []byte("#!/bin/sh\necho hello"), 0o755)

	m := &mockDocker{}
	mgr := NewManager(m, dir)

	if err := mgr.RunSetupScript("my-sandbox", "jvm"); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 1 || m.calls[0].method != "SandboxExec" {
		t.Fatalf("expected 1 SandboxExec call, got %v", m.calls)
	}
	script := strings.Join(m.calls[0].args, " ")
	if !strings.Contains(script, "echo hello") {
		t.Errorf("should execute setup.sh contents, got: %s", script)
	}
}

func TestRunSetupScriptNoFile(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, t.TempDir())

	if err := mgr.RunSetupScript("my-sandbox", "jvm"); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 0 {
		t.Errorf("should not exec when no setup.sh, got %v", m.calls)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestRunSetupScript -v`

Expected: FAIL — `RunSetupScript` not defined.

- [ ] **Step 3: Implement RunSetupScript**

Add this method to `internal/sandbox/sandbox.go`, after the `Create` method (after line 80):

```go
// RunSetupScript reads setup.sh from the template directory and executes it
// inside the container. If no setup.sh exists, it's a no-op.
func (m *Manager) RunSetupScript(sandboxName, template string) error {
	scriptPath := filepath.Join(m.templatesDir, template, "setup.sh")
	script, err := os.ReadFile(scriptPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading setup script: %w", err)
	}
	if _, err := m.docker.SandboxExec(sandboxName, "sh", "-c", string(script)); err != nil {
		return fmt.Errorf("running setup script: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestRunSetupScript -v`

Expected: PASS

- [ ] **Step 5: Run all sandbox tests to check for regressions**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -v`

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat: add RunSetupScript to sandbox manager

Reads setup.sh from the template directory and executes it inside
the container. No-op if no setup.sh exists. Same discovery pattern
as allowed-hosts.txt."
```

---

### Task 3: Update `create.go` — replace `environment.Setup()` with inline GITHUB_USERNAME + `RunSetupScript()`

**Files:**
- Modify: `internal/commands/create.go:1-73`

- [ ] **Step 1: Replace the environment.Setup() call and remove the import**

In `internal/commands/create.go`, make these changes:

1. Remove `"claudebox/internal/environment"` from the imports.

2. Replace lines 70-73 (the `environment.Setup()` call):

```go
	// 5. Setup environment
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}
```

with:

```go
	// 5. Setup environment
	if username := os.Getenv("GITHUB_USERNAME"); username != "" {
		script := fmt.Sprintf("printf 'export GITHUB_USERNAME=%%s\\n' %q >> /etc/sandbox-persistent.sh", username)
		if _, err := d.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
			return fmt.Errorf("setting GITHUB_USERNAME: %w", err)
		}
	}
	if err := mgr.RunSetupScript(sandboxName, template); err != nil {
		return err
	}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/andywang/Repos/claudebox && go build ./...`

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/commands/create.go
git commit -m "refactor: replace environment.Setup() with inline GITHUB_USERNAME + RunSetupScript

GITHUB_USERNAME is written directly to sandbox-persistent.sh.
Template-specific setup is delegated to setup.sh read from the
template directory."
```

---

### Task 4: Update `resume.go` — remove `environment.Setup()` call

**Files:**
- Modify: `internal/commands/resume.go:1-81`

- [ ] **Step 1: Remove environment.Setup() call and import**

In `internal/commands/resume.go`:

1. Remove `"claudebox/internal/environment"` from the imports.

2. Remove lines 78-81:

```go
	// Environment first, then credentials (matches Bash ordering)
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}
```

The code after the `Resuming sandbox` print should become:

```go
	fmt.Printf("Resuming sandbox: %s...\n", sandboxName)

	if err := credentials.Refresh(d, sandboxName); err != nil {
		return err
	}
	if err := mgr.WrapClaudeBinary(sandboxName); err != nil {
		return err
	}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/andywang/Repos/claudebox && go build ./...`

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/commands/resume.go
git commit -m "refactor: remove environment.Setup() from resume path

Environment is configured once at create time and persists across
sandbox stop/resume cycles."
```

---

### Task 5: Delete `internal/environment/` package

**Files:**
- Delete: `internal/environment/environment.go`
- Delete: `internal/environment/environment_test.go`

- [ ] **Step 1: Verify no remaining imports of the environment package**

Run: `cd /Users/andywang/Repos/claudebox && grep -r '"claudebox/internal/environment"' --include='*.go'`

Expected: No output (no files still importing it).

- [ ] **Step 2: Delete the package**

```bash
rm internal/environment/environment.go internal/environment/environment_test.go
rmdir internal/environment
```

- [ ] **Step 3: Verify everything still compiles and tests pass**

Run: `cd /Users/andywang/Repos/claudebox && go build ./... && go test ./...`

Expected: All builds and tests pass.

- [ ] **Step 4: Commit**

```bash
git add -A internal/environment/
git commit -m "refactor: delete internal/environment package

All responsibilities moved: GITHUB_USERNAME to create.go,
JVM setup to templates/jvm/setup.sh."
```
