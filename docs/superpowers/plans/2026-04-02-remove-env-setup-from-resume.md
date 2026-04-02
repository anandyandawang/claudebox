# Remove environment.Setup() from Resume Flow — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the redundant `environment.Setup()` call from the resume flow since env vars persist in the sandbox filesystem from create.

**Architecture:** Single-file change in `resume.go` — remove the call and unused import. Tests pass without modification since the mock doesn't track specific SandboxExec calls.

**Tech Stack:** Go, cobra CLI

---

### Task 1: Remove environment.Setup() from resume.go

**Files:**
- Modify: `internal/commands/resume.go:4-10` (imports), `internal/commands/resume.go:85-88` (Setup call)

- [ ] **Step 1: Run tests to establish green baseline**

Run: `cd /home/agent/workspace && go test ./internal/commands/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 2: Remove the environment.Setup() call and comment from resume.go**

In `internal/commands/resume.go`, remove lines 85-88:

```go
	// Environment first, then credentials (matches Bash ordering)
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}
```

The resulting code after `RefreshConfig` should flow directly to `credentials.Refresh`:

```go
	if err := mgr.RefreshConfig(sandboxName, claudeDir); err != nil {
		return err
	}

	if err := credentials.Refresh(d, sandboxName); err != nil {
		return err
	}
```

- [ ] **Step 3: Remove the unused `environment` import from resume.go**

In the import block of `internal/commands/resume.go`, remove:

```go
	"claudebox/internal/environment"
```

The import block should become:

```go
import (
	"bufio"
	"claudebox/internal/credentials"
	"claudebox/internal/docker"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)
```

- [ ] **Step 4: Run tests to verify everything still passes**

Run: `cd /home/agent/workspace && go test ./internal/commands/ -v -count=1`
Expected: All tests pass. No test changes needed — the mock `SandboxExec` is a generic no-op with no call-tracking assertions for environment.Setup.

- [ ] **Step 5: Run full test suite**

Run: `cd /home/agent/workspace && go test ./... -count=1`
Expected: All packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/commands/resume.go
git commit -m "fix: remove redundant environment.Setup() from resume flow

Env vars written to /etc/sandbox-persistent.sh during create persist
across sandbox stop/start cycles. Re-running Setup on resume was
unnecessary — it truncated and re-wrote the same values each time."
```
