# Sandbox Branch Naming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use the sandbox instance ID (`MMDD-cat-hash`) as the git branch name instead of the disconnected `sandbox-YYYYMMDD-HHMMSS` format.

**Architecture:** Extract a `GenerateSandboxID(template)` function that returns `MMDD-cat-hash`. Change `GenerateSandboxName` to accept the sandbox ID instead of the template. Delete `GenerateSessionID`. Update callers and tests.

**Tech Stack:** Go, `go test`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/sandbox/naming.go` | Modify | Add `GenerateSandboxID`, update `GenerateSandboxName` signature, delete `GenerateSessionID` |
| `internal/sandbox/naming_test.go` | Modify | Update tests for new API |
| `internal/commands/create.go` | Modify | Use `GenerateSandboxID` + pass to both sandbox name and session ID |
| `tests/integration/helpers_test.go` | Modify | Use `GenerateSandboxID` instead of `GenerateSessionID` |
| `tests/integration/filesystem_test.go` | Modify | Update branch pattern check |

---

### Task 1: Add `GenerateSandboxID` with TDD

**Files:**
- Modify: `internal/sandbox/naming_test.go:100-106`
- Modify: `internal/sandbox/naming.go:67-70`

- [ ] **Step 1: Write the failing test for `GenerateSandboxID`**

In `internal/sandbox/naming_test.go`, replace `TestGenerateSessionID` (lines 100-106) with:

```go
func TestGenerateSandboxID(t *testing.T) {
	id := GenerateSandboxID("jvm")
	// Format: MMDD-cat(5)-hash(2), e.g. "0401-chonk-f3"
	pattern := `^\d{4}-[a-z]{5}-[0-9a-f]{2}$`
	if matched, _ := regexp.MatchString(pattern, id); !matched {
		t.Errorf("GenerateSandboxID = %q, want match %s", id, pattern)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestGenerateSandboxID -v`
Expected: compilation error — `GenerateSandboxID` undefined.

- [ ] **Step 3: Implement `GenerateSandboxID`**

In `internal/sandbox/naming.go`, replace `GenerateSessionID` (lines 67-70) with:

```go
// GenerateSandboxID returns a unique sandbox instance ID: MMDD-cat(5)-hash(2).
func GenerateSandboxID(template string) string {
	cat := randomCatName()
	iHash := instanceHash(template, cat)
	mmdd := time.Now().Format("0102")
	return fmt.Sprintf("%s-%s-%s", mmdd, cat, iHash)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestGenerateSandboxID -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/sandbox/naming.go internal/sandbox/naming_test.go
git commit -m "feat: add GenerateSandboxID, replace GenerateSessionID"
```

---

### Task 2: Refactor `GenerateSandboxName` to accept sandbox ID

**Files:**
- Modify: `internal/sandbox/naming.go:90-97`
- Modify: `internal/sandbox/naming_test.go` (multiple test functions)

- [ ] **Step 1: Update `GenerateSandboxName` signature and body**

In `internal/sandbox/naming.go`, replace the current `GenerateSandboxName` (lines 90-97) with:

```go
// GenerateSandboxName returns: <wshash(2)>-<workspace(12)>.<sandboxID>
func GenerateSandboxName(workspacePath, sandboxID string) string {
	return WorkspacePrefix(workspacePath) + sandboxID
}
```

- [ ] **Step 2: Update `TestGenerateSandboxName`**

Replace the test at lines 26-39 with:

```go
func TestGenerateSandboxName(t *testing.T) {
	id := GenerateSandboxID("jvm")
	name := GenerateSandboxName("/path/to/my-project", id)

	// Format: <wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
	pattern := `^[0-9a-f]{2}-[a-zA-Z0-9_.-]{1,12}\.\d{4}-[a-z]{5}-[0-9a-f]{2}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("GenerateSandboxName = %q, want match %s", name, pattern)
	}

	// Max length
	if len(name) > maxSandboxNameLen {
		t.Errorf("GenerateSandboxName length = %d, want <= %d", len(name), maxSandboxNameLen)
	}
}
```

- [ ] **Step 3: Update `TestGenerateSandboxNameTruncatesLongWorkspace`**

Replace lines 41-56 with:

```go
func TestGenerateSandboxNameTruncatesLongWorkspace(t *testing.T) {
	id := GenerateSandboxID("jvm")
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", id)

	// "lambda-jpm-clearings" is 20 chars, should truncate to 12
	dotIdx := strings.Index(name, ".")
	if dotIdx == -1 {
		t.Fatalf("no dot in name: %q", name)
	}
	ws := name[3:dotIdx] // skip wshash(2) + dash
	if len(ws) > 12 {
		t.Errorf("workspace portion %q exceeds 12 chars", ws)
	}
}
```

- [ ] **Step 4: Update `TestGenerateSandboxNameTruncatesLongTemplate`**

Replace lines 58-66 with:

```go
func TestGenerateSandboxNameTruncatesLongTemplate(t *testing.T) {
	id := GenerateSandboxID("kotlin-spring")
	name := GenerateSandboxName("/path/to/myapp", id)

	if len(name) > maxSandboxNameLen {
		t.Errorf("name length = %d, want <= %d", len(name), maxSandboxNameLen)
	}
}
```

- [ ] **Step 5: Update `TestDegenerateWorkspaceNames`**

In the test at lines 68-98, replace the inner loop body (line 80):

```go
name := GenerateSandboxName(tt.path, "jvm")
```

with:

```go
name := GenerateSandboxName(tt.path, GenerateSandboxID("jvm"))
```

- [ ] **Step 6: Update `TestGenerateSandboxNameMaxLength`**

Replace the inner loop body at line 173:

```go
name := GenerateSandboxName("/path/to/abcdefghijklmnopqrstuvwxyz", "long-template-name")
```

with:

```go
name := GenerateSandboxName("/path/to/abcdefghijklmnopqrstuvwxyz", GenerateSandboxID("long-template-name"))
```

- [ ] **Step 7: Update `TestGenerateSandboxNameSocketPathFits`**

Replace line 183:

```go
name := GenerateSandboxName("/path/to/some-long-workspace-name", "jvm")
```

with:

```go
name := GenerateSandboxName("/path/to/some-long-workspace-name", GenerateSandboxID("jvm"))
```

- [ ] **Step 8: Update `TestWorkspacePrefix`**

Replace line 200:

```go
name := GenerateSandboxName("/path/to/lambda-jpm-clearings", "jvm")
```

with:

```go
name := GenerateSandboxName("/path/to/lambda-jpm-clearings", GenerateSandboxID("jvm"))
```

- [ ] **Step 9: Update `TestWorkspacePrefixMatchesDifferentTemplates`**

Replace lines 228-229:

```go
nameJVM := GenerateSandboxName(ws, "jvm")
nameKotlin := GenerateSandboxName(ws, "kotlin-spring")
```

with:

```go
nameJVM := GenerateSandboxName(ws, GenerateSandboxID("jvm"))
nameKotlin := GenerateSandboxName(ws, GenerateSandboxID("kotlin-spring"))
```

- [ ] **Step 10: Update `TestWorkspacePrefixIsolatesTruncationCollisions`**

Replace lines 273-274:

```go
namesA = append(namesA, GenerateSandboxName(wsA, "jvm"))
namesB = append(namesB, GenerateSandboxName(wsB, "jvm"))
```

with:

```go
namesA = append(namesA, GenerateSandboxName(wsA, GenerateSandboxID("jvm")))
namesB = append(namesB, GenerateSandboxName(wsB, GenerateSandboxID("jvm")))
```

- [ ] **Step 11: Run all unit tests**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -v`
Expected: all PASS

- [ ] **Step 12: Commit**

```bash
git add internal/sandbox/naming.go internal/sandbox/naming_test.go
git commit -m "refactor: GenerateSandboxName accepts sandboxID instead of template"
```

---

### Task 3: Update `create.go` to use new API

**Files:**
- Modify: `internal/commands/create.go:55-65`

- [ ] **Step 1: Update the create flow**

In `internal/commands/create.go`, replace lines 54-65:

```go
	// 3. Generate names
	sessionID := sandbox.GenerateSessionID()
	sandboxName := sandbox.GenerateSandboxName(workspace, template)
	claudeDir := os.Getenv("HOME") + "/.claude"

	// 4. Create sandbox
	fmt.Printf("Creating sandbox: %s...\n", sandboxName)
	if err := mgr.Create(sandboxName, sandbox.CreateOpts{
		ImageName: imageName,
		Workspace: workspace,
		ClaudeDir: claudeDir,
		SessionID: sessionID,
```

with:

```go
	// 3. Generate names
	sandboxID := sandbox.GenerateSandboxID(template)
	sandboxName := sandbox.GenerateSandboxName(workspace, sandboxID)
	claudeDir := os.Getenv("HOME") + "/.claude"

	// 4. Create sandbox
	fmt.Printf("Creating sandbox: %s...\n", sandboxName)
	if err := mgr.Create(sandboxName, sandbox.CreateOpts{
		ImageName: imageName,
		Workspace: workspace,
		ClaudeDir: claudeDir,
		SessionID: sandboxID,
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/agent/workspace && go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/commands/create.go
git commit -m "feat: use sandbox ID as branch name in create flow"
```

---

### Task 4: Update integration tests

**Files:**
- Modify: `tests/integration/helpers_test.go:51-54`
- Modify: `tests/integration/filesystem_test.go:26-34`

- [ ] **Step 1: Update `createTestSandbox` in `helpers_test.go`**

Replace lines 52-54:

```go
	sessionID := sandbox.GenerateSessionID()
	name := sandbox.GenerateSandboxName(workspace, template)
```

with:

```go
	sandboxID := sandbox.GenerateSandboxID(template)
	name := sandbox.GenerateSandboxName(workspace, sandboxID)
```

And replace line 60:

```go
		SessionID: sessionID,
```

with:

```go
		SessionID: sandboxID,
```

- [ ] **Step 2: Update branch pattern check in `filesystem_test.go`**

Add `"regexp"` to the imports (line 4 area — it already imports `"strings"` and `"testing"`).

Replace the branch check at lines 26-34:

```go
	t.Run("git branch matches sandbox pattern", func(t *testing.T) {
		branch, err := testDocker.SandboxExec(name, "git", "-C", sandbox.SandboxWorkspace, "branch", "--show-current")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(branch, "sandbox-") {
			t.Errorf("branch %q should start with sandbox-", branch)
		}
	})
```

with:

```go
	t.Run("git branch matches sandbox ID pattern", func(t *testing.T) {
		branch, err := testDocker.SandboxExec(name, "git", "-C", sandbox.SandboxWorkspace, "branch", "--show-current")
		if err != nil {
			t.Fatal(err)
		}
		pattern := `^\d{4}-[a-z]{5}-[0-9a-f]{2}$`
		if matched, _ := regexp.MatchString(pattern, strings.TrimSpace(branch)); !matched {
			t.Errorf("branch %q should match pattern %s", branch, pattern)
		}
	})
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/agent/workspace && go build ./... && go vet ./...`
Expected: no errors (integration tests won't run without `--tags=integration` and Docker, but they must compile)

- [ ] **Step 4: Commit**

```bash
git add tests/integration/helpers_test.go tests/integration/filesystem_test.go
git commit -m "test: update integration tests for sandbox ID branch naming"
```

---

### Task 5: Final verification

- [ ] **Step 1: Run all unit tests**

Run: `cd /home/agent/workspace && go test ./... -v`
Expected: all PASS

- [ ] **Step 2: Verify no remaining references to `GenerateSessionID`**

Run: `grep -r "GenerateSessionID\|SessionID.*sandbox-" --include="*.go" /home/agent/workspace/`
Expected: no matches (the `SessionID` field in the struct is fine — it's the old `sandbox-` format references we're checking for)

- [ ] **Step 3: Commit if any cleanup was needed, otherwise done**
