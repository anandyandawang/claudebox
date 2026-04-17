# Import Host Git Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import host's `--global` `user.name` and `user.email` into the sandbox's global gitconfig on create, and add integration coverage for the three observable contracts of `environment.Setup()`.

**Architecture:** Add a replaceable `readGitIdentityFn` seam to `internal/environment/environment.go` (mirrors `readKeychainFn` in `credentials/keychain.go`) that reads each value via `git config --global <key>` on the host. `Setup()` calls it after the `GITHUB_USERNAME` block and writes each non-empty value into the sandbox with `git config --global <key> <value>`. Unit tests use the existing `mockDocker` and swap `readGitIdentityFn`. Integration tests query `git config`, sourced `/etc/sandbox-persistent.sh`, and sourced `JAVA_TOOL_OPTIONS` inside the running sandbox.

**Tech Stack:** Go, `os/exec`, Docker Desktop sandboxes.

**Spec:** `docs/superpowers/specs/2026-04-17-import-host-git-identity-design.md`

---

## File Structure

**Modified:**
- `internal/environment/environment.go` — add `readGitIdentityFn` var + `readGitIdentity` / `readGitConfigGlobal` helpers; call them from `Setup()`.
- `internal/environment/environment_test.go` — add `withGitIdentity` helper + four unit tests (both/only-name/only-email/neither).
- `tests/integration/filesystem_test.go` — add three subtests to `TestFilesystemLayout`.

**No files created.** Existing pattern is a single file per package for environment concerns.

---

## Task 1: Unit TDD — import host git identity in Setup()

**Files:**
- Modify: `internal/environment/environment.go`
- Test: `internal/environment/environment_test.go`

- [ ] **Step 1: Add `withGitIdentity` helper + `hasGitConfigCall` helper to test file**

Open `internal/environment/environment_test.go`. Add these helpers after the `mockDocker` definitions (before `TestSetupExportsGitHubUsername`):

```go
// withGitIdentity swaps readGitIdentityFn for the duration of a test.
func withGitIdentity(t *testing.T, name, email string) {
	t.Helper()
	orig := readGitIdentityFn
	readGitIdentityFn = func() (string, string) { return name, email }
	t.Cleanup(func() { readGitIdentityFn = orig })
}

// hasGitConfigCall returns true if execCalls contains a `git config --global <key> <value>` call.
func hasGitConfigCall(calls [][]string, key, value string) bool {
	for _, c := range calls {
		// c = [sandboxName, "git", "config", "--global", <key>, <value>]
		if len(c) >= 6 && c[1] == "git" && c[2] == "config" && c[3] == "--global" && c[4] == key && c[5] == value {
			return true
		}
	}
	return false
}

// hasAnyGitConfigCall returns true if execCalls contains any `git config --global <key> ...` call.
func hasAnyGitConfigCall(calls [][]string, key string) bool {
	for _, c := range calls {
		if len(c) >= 5 && c[1] == "git" && c[2] == "config" && c[3] == "--global" && c[4] == key {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Add four unit tests**

At the end of `internal/environment/environment_test.go`, append:

```go
func TestSetupImportsGitIdentityBoth(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "Alice", "alice@example.com")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if !hasGitConfigCall(md.execCalls, "user.name", "Alice") {
		t.Error("should set git user.name=Alice")
	}
	if !hasGitConfigCall(md.execCalls, "user.email", "alice@example.com") {
		t.Error("should set git user.email=alice@example.com")
	}
}

func TestSetupImportsGitIdentityOnlyName(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "Alice", "")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if !hasGitConfigCall(md.execCalls, "user.name", "Alice") {
		t.Error("should set git user.name=Alice")
	}
	if hasAnyGitConfigCall(md.execCalls, "user.email") {
		t.Error("should not set git user.email when host has none")
	}
}

func TestSetupImportsGitIdentityOnlyEmail(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "", "alice@example.com")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if hasAnyGitConfigCall(md.execCalls, "user.name") {
		t.Error("should not set git user.name when host has none")
	}
	if !hasGitConfigCall(md.execCalls, "user.email", "alice@example.com") {
		t.Error("should set git user.email=alice@example.com")
	}
}

func TestSetupImportsGitIdentityNeither(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "", "")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if hasAnyGitConfigCall(md.execCalls, "user.name") {
		t.Error("should not set git user.name when host has none")
	}
	if hasAnyGitConfigCall(md.execCalls, "user.email") {
		t.Error("should not set git user.email when host has none")
	}
}
```

- [ ] **Step 3: Run unit tests to verify they fail to compile**

Run: `cd /home/agent/workspace && make test-unit`

Expected: FAIL with compile error mentioning `readGitIdentityFn` undefined in `internal/environment`.

- [ ] **Step 4: Add `readGitIdentityFn` + helpers to `environment.go`**

Open `internal/environment/environment.go`. Replace the imports block:

```go
import (
	"claudebox/internal/docker"
	"fmt"
	"os"
)
```

with:

```go
import (
	"claudebox/internal/docker"
	"fmt"
	"os"
	"os/exec"
	"strings"
)
```

Then, immediately after the import block and before `func Setup`, add:

```go
// readGitIdentityFn returns the host's --global user.name and user.email.
// Empty strings on error or unset. Replaceable for testing.
var readGitIdentityFn = readGitIdentity

func readGitIdentity() (name, email string) {
	return readGitConfigGlobal("user.name"), readGitConfigGlobal("user.email")
}

func readGitConfigGlobal(key string) string {
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

- [ ] **Step 5: Wire git identity import into Setup()**

In `internal/environment/environment.go`, find the `Setup` function. After the `GITHUB_USERNAME` block (after its closing `}`) and before the `// Configure JVM proxy and import CA cert` comment, insert:

```go
	// Import host git identity into sandbox's global gitconfig.
	// Silently skip either value if unset on host.
	gitName, gitEmail := readGitIdentityFn()
	if gitName != "" {
		if _, err := d.SandboxExec(sandboxName, "git", "config", "--global", "user.name", gitName); err != nil {
			return fmt.Errorf("setting git user.name: %w", err)
		}
	}
	if gitEmail != "" {
		if _, err := d.SandboxExec(sandboxName, "git", "config", "--global", "user.email", gitEmail); err != nil {
			return fmt.Errorf("setting git user.email: %w", err)
		}
	}
```

- [ ] **Step 6: Run unit tests to verify they pass**

Run: `cd /home/agent/workspace && make test-unit`

Expected: PASS. All four new tests pass, plus the existing `TestSetupExportsGitHubUsername` and `TestSetupConfiguresJVMProxy`. (The existing tests do not stub `readGitIdentityFn`, so they invoke real `git` on the host; `readGitConfigGlobal` swallows errors, so either real values or empty strings flow through without affecting the existing assertions.)

- [ ] **Step 7: Commit**

```bash
cd /home/agent/workspace
git add internal/environment/environment.go internal/environment/environment_test.go
git commit -m "$(cat <<'EOF'
feat: import host git identity into sandbox gitconfig

environment.Setup() now reads the host's --global user.name and
user.email via `git config` and writes each non-empty value into the
sandbox's global gitconfig. Silently skips either value that is unset
on the host, matching the GITHUB_USERNAME pattern.

Spec: docs/superpowers/specs/2026-04-17-import-host-git-identity-design.md
EOF
)"
```

---

## Task 2: Integration subtests for environment.Setup() contracts

**Files:**
- Modify: `tests/integration/filesystem_test.go`

This task adds three subtests to `TestFilesystemLayout`. All three close a pre-existing gap: `environment.Setup()` runs in every integration test via `createTestSandbox`, but nothing verifies its side effects inside the container.

- [ ] **Step 1: Add imports**

Open `tests/integration/filesystem_test.go`. The current imports are:

```go
import (
	"claudebox/internal/sandbox"
	"os"
	"os/exec"
	"strings"
	"testing"
)
```

Replace with:

```go
import (
	"claudebox/internal/sandbox"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Add `host git identity imported into sandbox` subtest**

In `tests/integration/filesystem_test.go`, inside the `TestFilesystemLayout` function, after the `t.Run("host paths rewritten in claude config", ...)` block and before the closing `}` of `TestFilesystemLayout`, append:

```go
	t.Run("host git identity imported into sandbox", func(t *testing.T) {
		hostNameOut, _ := exec.Command("git", "config", "--global", "user.name").Output()
		hostEmailOut, _ := exec.Command("git", "config", "--global", "user.email").Output()
		hostName := strings.TrimSpace(string(hostNameOut))
		hostEmail := strings.TrimSpace(string(hostEmailOut))
		if hostName == "" && hostEmail == "" {
			t.Skip("host has no global git identity")
		}

		if hostName != "" {
			got, err := testDocker.SandboxExec(sb.name, "git", "config", "--global", "user.name")
			if err != nil {
				t.Fatalf("reading sandbox user.name: %v", err)
			}
			if strings.TrimSpace(got) != hostName {
				t.Errorf("sandbox user.name = %q, want %q", strings.TrimSpace(got), hostName)
			}
		}
		if hostEmail != "" {
			got, err := testDocker.SandboxExec(sb.name, "git", "config", "--global", "user.email")
			if err != nil {
				t.Fatalf("reading sandbox user.email: %v", err)
			}
			if strings.TrimSpace(got) != hostEmail {
				t.Errorf("sandbox user.email = %q, want %q", strings.TrimSpace(got), hostEmail)
			}
		}
	})
```

- [ ] **Step 3: Add `GITHUB_USERNAME exported in sandbox-persistent.sh` subtest**

Append directly after the block added in Step 2:

```go
	t.Run("GITHUB_USERNAME exported in sandbox-persistent.sh", func(t *testing.T) {
		hostValue := os.Getenv("GITHUB_USERNAME")
		if hostValue == "" {
			t.Skip("GITHUB_USERNAME not set on host")
		}

		got, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`. /etc/sandbox-persistent.sh && printf %s "$GITHUB_USERNAME"`)
		if err != nil {
			t.Fatalf("sourcing sandbox-persistent.sh: %v", err)
		}
		if strings.TrimSpace(got) != hostValue {
			t.Errorf("sandbox GITHUB_USERNAME = %q, want %q", strings.TrimSpace(got), hostValue)
		}
	})
```

- [ ] **Step 4: Add `JAVA_TOOL_OPTIONS written when HTTPS_PROXY is set on host` subtest**

Append directly after the block added in Step 3:

```go
	t.Run("JAVA_TOOL_OPTIONS written when HTTPS_PROXY is set on host", func(t *testing.T) {
		proxy := os.Getenv("HTTPS_PROXY")
		if proxy == "" {
			t.Skip("HTTPS_PROXY not set on host")
		}

		u, err := url.Parse(proxy)
		if err != nil || u.Hostname() == "" || u.Port() == "" {
			t.Fatalf("cannot parse HTTPS_PROXY=%q: %v", proxy, err)
		}
		host, port := u.Hostname(), u.Port()

		got, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`. /etc/sandbox-persistent.sh && printf %s "$JAVA_TOOL_OPTIONS"`)
		if err != nil {
			t.Fatalf("sourcing sandbox-persistent.sh: %v", err)
		}
		opts := strings.TrimSpace(got)

		want := []string{
			fmt.Sprintf("-Dhttp.proxyHost=%s", host),
			fmt.Sprintf("-Dhttp.proxyPort=%s", port),
			fmt.Sprintf("-Dhttps.proxyHost=%s", host),
			fmt.Sprintf("-Dhttps.proxyPort=%s", port),
		}
		for _, w := range want {
			if !strings.Contains(opts, w) {
				t.Errorf("JAVA_TOOL_OPTIONS missing %q; got: %q", w, opts)
			}
		}
	})
```

- [ ] **Step 5: Run integration tests**

Run: `cd /home/agent/workspace && make test-integration`

Expected: PASS. Subtests that depend on host env (`GITHUB_USERNAME`, `HTTPS_PROXY`) may `SKIP`; that's a pass outcome. The `host git identity imported into sandbox` subtest should pass if the host has a global `user.name` or `user.email` (most dev machines do) and skip otherwise.

Note: `make test-integration` requires Docker Desktop with sandbox support and takes minutes (builds the `jvm` image, creates a sandbox). If it fails with "docker sandbox not available," the test environment lacks Docker sandbox support — run on a machine that has it.

- [ ] **Step 6: Commit**

```bash
cd /home/agent/workspace
git add tests/integration/filesystem_test.go
git commit -m "$(cat <<'EOF'
test: assert environment.Setup side effects end-to-end

Add three subtests to TestFilesystemLayout verifying the observable
contracts of environment.Setup():
  - host git user.name/user.email propagate into sandbox gitconfig
  - GITHUB_USERNAME lands in /etc/sandbox-persistent.sh
  - JAVA_TOOL_OPTIONS reflects host HTTPS_PROXY (JVM proxy)

Each subtest t.Skips when its host precondition is absent.

Spec: docs/superpowers/specs/2026-04-17-import-host-git-identity-design.md
EOF
)"
```

---

## Spec Coverage Self-Check

- ✅ Placement in `internal/environment/environment.go`, called from `Setup()` → Task 1 Step 5.
- ✅ Read host values via `git config --global user.name/user.email` → Task 1 Step 4.
- ✅ Write sandbox values with `git config --global` (separate exec args, no shell escape) → Task 1 Step 5.
- ✅ `readGitIdentityFn` seam mirroring `readKeychainFn` → Task 1 Step 4.
- ✅ Silently skip unset values → Task 1 Step 5 (`if gitName != ""` / `if gitEmail != ""`); Task 1 Step 2 (`TestSetupImportsGitIdentityNeither` + OnlyName / OnlyEmail).
- ✅ Return error from Setup() on sandbox exec failure → Task 1 Step 5 (`return fmt.Errorf(...)`).
- ✅ Unit tests: four permutations (both / only name / only email / neither) → Task 1 Step 2.
- ✅ Integration subtest 1: host git identity → Task 2 Step 2.
- ✅ Integration subtest 2: GITHUB_USERNAME in `/etc/sandbox-persistent.sh` → Task 2 Step 3.
- ✅ Integration subtest 3: JAVA_TOOL_OPTIONS from HTTPS_PROXY → Task 2 Step 4.
- ✅ Keytool branch explicitly uncovered at integration level (unit-only) → spec notes; no plan task.
- ✅ Out of scope: no signing/rebase keys, no resume-refresh, no system/local merge → plan touches none of these.
