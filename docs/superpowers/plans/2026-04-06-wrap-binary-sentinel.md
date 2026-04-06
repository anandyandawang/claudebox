# Wrap Binary Sentinel Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `WrapClaudeBinary` so auto-updated Claude binaries replace the stale `claude-real` instead of being silently discarded.

**Architecture:** Add a `CLAUDEBOX_WRAPPER` sentinel comment to the wrapper script. Before wrapping, check if the current `claude` binary contains the sentinel. If it doesn't (auto-update replaced it), move the new binary to `claude-real` before writing the wrapper.

**Tech Stack:** Go, shell scripting, `go test`

---

### Task 1: Fix the wrapper script in sandbox.go

**Files:**
- Modify: `internal/sandbox/sandbox.go:252-268`

- [ ] **Step 1: Update the WrapClaudeBinary script string**

Replace the `WrapClaudeBinary` method body at `internal/sandbox/sandbox.go:253-267` with:

```go
func (m *Manager) WrapClaudeBinary(sandboxName string) error {
	script := fmt.Sprintf(`CLAUDE_BIN=$(which claude)
if [ ! -f "${CLAUDE_BIN}-real" ]; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
elif ! grep -q 'CLAUDEBOX_WRAPPER' "$CLAUDE_BIN"; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
fi
sudo tee "$CLAUDE_BIN" > /dev/null << 'WRAPPER'
#!/bin/bash
# CLAUDEBOX_WRAPPER
cd %s
exec "$(dirname "$0")/claude-real" "$@"
WRAPPER
sudo chmod +x "$CLAUDE_BIN"`, SandboxWorkspace)
	if _, err := m.docker.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
		return fmt.Errorf("wrapping claude binary: %w", err)
	}
	return nil
}
```

Changes from before:
1. Added `elif ! grep -q 'CLAUDEBOX_WRAPPER' "$CLAUDE_BIN"` — detects auto-update replacement.
2. Added `# CLAUDEBOX_WRAPPER` comment to the wrapper body — the sentinel grep checks for.

- [ ] **Step 2: Run unit tests**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestWrapClaudeBinary -v`
Expected: PASS — the existing test only checks that `claude-real` appears in the script, which still holds.

- [ ] **Step 3: Verify sentinel appears in the script**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestWrapClaudeBinary -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/sandbox/sandbox.go
git commit -m "fix: add sentinel to WrapClaudeBinary to detect auto-updates

The idempotent guard skipped mv when claude-real existed, silently
discarding auto-updated binaries. Now checks for a CLAUDEBOX_WRAPPER
sentinel — if claude lacks it, the binary was replaced by an
auto-update and gets moved to claude-real."
```

---

### Task 2: Update the unit test to verify sentinel content

**Files:**
- Modify: `internal/sandbox/sandbox_test.go:369-383`

- [ ] **Step 1: Update TestWrapClaudeBinary to assert on sentinel**

Replace the `TestWrapClaudeBinary` function at `internal/sandbox/sandbox_test.go:369-383` with:

```go
func TestWrapClaudeBinary(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, "/templates")

	if err := mgr.WrapClaudeBinary("my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 1 || m.calls[0].method != "SandboxExec" {
		t.Errorf("WrapClaudeBinary: got %v", m.calls)
	}
	script := strings.Join(m.calls[0].args, " ")
	if !strings.Contains(script, "claude-real") {
		t.Error("WrapClaudeBinary script should reference claude-real")
	}
	if !strings.Contains(script, "CLAUDEBOX_WRAPPER") {
		t.Error("WrapClaudeBinary script should contain CLAUDEBOX_WRAPPER sentinel")
	}
	if !strings.Contains(script, "grep -q") {
		t.Error("WrapClaudeBinary script should use grep to detect auto-updates")
	}
}
```

- [ ] **Step 2: Run the updated test**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestWrapClaudeBinary -v`
Expected: PASS

- [ ] **Step 3: Run all sandbox unit tests to check for regressions**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/sandbox/sandbox_test.go
git commit -m "test: assert sentinel and grep guard in WrapClaudeBinary unit test"
```

---

### Task 3: Fix the integration test for auto-update detection

**Files:**
- Modify: `tests/integration/filesystem_test.go:53-95`

- [ ] **Step 1: Update "re-wrap after binary replacement restores wrapper" test**

Replace the test at `tests/integration/filesystem_test.go:53-95` with:

```go
	t.Run("re-wrap after binary replacement restores wrapper", func(t *testing.T) {
		// Simulate Claude Code auto-update: overwrite the wrapper with a fake binary.
		_, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`sudo tee "$(which claude)" > /dev/null <<'BIN'
#!/bin/bash
echo "I am a new claude binary"
BIN`)
		if err != nil {
			t.Fatal(err)
		}

		// Re-wrap (as resume would do). Sentinel is missing from the fake binary,
		// so it should be moved to claude-real (replacing the old one).
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		// Wrapper should be restored.
		out, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cd /home/agent/workspace") {
			t.Errorf("wrapper should be restored with cd, got: %s", out)
		}
		if !strings.Contains(out, "CLAUDEBOX_WRAPPER") {
			t.Errorf("wrapper should contain sentinel, got: %s", out)
		}

		// claude-real should now be the NEW fake binary (sentinel detected the replacement).
		realAfter, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(realAfter, "new claude binary") {
			t.Errorf("claude-real should be the auto-updated binary, got: %s", realAfter)
		}
	})
```

Key change: Previously asserted `claude-real` was **unchanged** (old binary preserved). Now asserts `claude-real` contains the **new** fake binary — the sentinel detected the replacement and moved it.

- [ ] **Step 2: Verify integration test compiles**

Run: `cd /home/agent/workspace && go vet -tags=integration ./tests/integration/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add tests/integration/filesystem_test.go
git commit -m "test: fix auto-update integration test to expect new binary in claude-real

The old test asserted claude-real was unchanged after re-wrap, which
validated the bug. With sentinel detection, the auto-updated binary
should be moved to claude-real, replacing the stale one."
```

---

### Task 4: Add idempotency integration test

**Files:**
- Modify: `tests/integration/filesystem_test.go` (add new test after the "re-wrap after full binary replacement" test, around line 138)

- [ ] **Step 1: Add "re-wrap is idempotent when wrapper intact" test**

Insert after the closing `})` of the `"re-wrap after full binary replacement uses mv path"` test (line 138):

```go
	t.Run("re-wrap is idempotent when wrapper intact", func(t *testing.T) {
		// Ensure wrapper is in place from a prior test.
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		// Capture claude-real before a second re-wrap.
		realBefore, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal(err)
		}

		// Re-wrap again — wrapper has the sentinel, so claude-real should NOT change.
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		realAfter, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal(err)
		}
		if realAfter != realBefore {
			t.Errorf("claude-real should be unchanged when wrapper is intact:\nbefore: %s\nafter: %s", realBefore, realAfter)
		}
	})
```

- [ ] **Step 2: Verify integration test compiles**

Run: `cd /home/agent/workspace && go vet -tags=integration ./tests/integration/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add tests/integration/filesystem_test.go
git commit -m "test: add idempotency integration test for WrapClaudeBinary sentinel"
```

---

### Task 5: Update the existing spec for accuracy

**Files:**
- Modify: `docs/superpowers/specs/2026-04-02-remove-env-setup-from-resume-design.md:44-46`

- [ ] **Step 1: Update the "WrapClaudeBinary stays in resume" section**

Replace line 46 of `docs/superpowers/specs/2026-04-02-remove-env-setup-from-resume-design.md`:

```
- `mgr.WrapClaudeBinary()` must run on resume. The wrapper script persists across stop/start, but Claude Code auto-updates can replace the binary at the `claude` path, overwriting the wrapper with a fresh binary. Without re-wrapping on resume, the sandbox boots in the empty mount directory instead of the workspace. The idempotent guard (`if [ ! -f "${CLAUDE_BIN}-real" ]`) ensures re-wrapping is safe when the wrapper is still intact.
```

With:

```
- `mgr.WrapClaudeBinary()` must run on resume. The wrapper script persists across stop/start, but Claude Code auto-updates can replace the binary at the `claude` path, overwriting the wrapper with a fresh binary. Without re-wrapping on resume, the sandbox boots in the empty mount directory instead of the workspace. A `CLAUDEBOX_WRAPPER` sentinel in the wrapper script detects whether `claude` is still the wrapper or has been replaced by an auto-update. If replaced, the new binary is moved to `claude-real` before re-writing the wrapper — ensuring the sandbox picks up the updated binary.
```

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-04-02-remove-env-setup-from-resume-design.md
git commit -m "docs: update spec to reflect sentinel-based auto-update detection"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run all unit tests**

Run: `cd /home/agent/workspace && go test ./... -v`
Expected: All PASS

- [ ] **Step 2: Verify integration tests compile**

Run: `cd /home/agent/workspace && go vet -tags=integration ./tests/integration/`
Expected: No errors

- [ ] **Step 3: Review the git log**

Run: `git log --oneline -6`
Expected: 5 new commits (Tasks 1-5) on top of the spec commit.
