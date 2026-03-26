# Sandbox Naming Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign sandbox naming to fit within the macOS Unix socket path limit (103 chars) while keeping names human-readable with meme cat names.

**Architecture:** Fixed-length name format `<wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>` (max 29 chars). Two SHA-256 hashes: a stable workspace hash for prefix matching, and an instance hash with microsecond timestamp for uniqueness. Workspace prefix matching uses everything left of and including the dot.

**Tech Stack:** Go stdlib (`crypto/sha256`, `encoding/hex`, `math/rand`, `time`)

---

### Task 1: Add helpers and cat name list to naming.go

**Files:**
- Modify: `internal/sandbox/naming.go`
- Modify: `internal/sandbox/naming_test.go`

- [ ] **Step 1: Write failing tests for truncateClean**

Add to `internal/sandbox/naming_test.go`:

```go
func TestTruncateClean(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello-world", 5, "hello"},
		{"hello-", 5, "hello"},
		{"hello-", 6, "hello"},
		{"ab-cd-ef", 5, "ab-cd"},
		{"ab---", 5, "ab"},
		{"ab...", 5, "ab"},
		{"a", 12, "a"},
		{"abcdefghijklmnop", 12, "abcdefghijkl"},
		{"abcdefghijkl-", 12, "abcdefghijkl"},
		{"abcdefghijkl.", 12, "abcdefghijkl"},
	}
	for _, tt := range tests {
		got := truncateClean(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncateClean(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestTruncateClean -v`
Expected: FAIL — `truncateClean` not defined

- [ ] **Step 3: Implement truncateClean**

Add to `internal/sandbox/naming.go`:

```go
// truncateClean truncates s to max chars and strips trailing hyphens and dots.
func truncateClean(s string, max int) string {
	if len(s) > max {
		s = s[:max]
	}
	return strings.TrimRight(s, "-.")
}
```

Add `"strings"` to the imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestTruncateClean -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for workspaceHash**

Add to `internal/sandbox/naming_test.go`:

```go
func TestWorkspaceHash(t *testing.T) {
	// Deterministic: same input → same output
	h1 := workspaceHash("lambda-jpm-clearings")
	h2 := workspaceHash("lambda-jpm-clearings")
	if h1 != h2 {
		t.Errorf("workspaceHash not deterministic: %q != %q", h1, h2)
	}
	// Length is always 2
	if len(h1) != 2 {
		t.Errorf("workspaceHash length = %d, want 2", len(h1))
	}
	// Different inputs → different outputs (probabilistic but reliable for these inputs)
	h3 := workspaceHash("lambda-jpm-clients")
	if h1 == h3 {
		t.Errorf("workspaceHash collision: %q and %q both produce %q", "lambda-jpm-clearings", "lambda-jpm-clients", h1)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestWorkspaceHash -v`
Expected: FAIL — `workspaceHash` not defined

- [ ] **Step 7: Implement workspaceHash and instanceHash**

Add to `internal/sandbox/naming.go`:

```go
// workspaceHash returns the first 2 hex chars of SHA-256 of the full workspace name.
func workspaceHash(fullWorkspace string) string {
	h := sha256.Sum256([]byte(fullWorkspace))
	return hex.EncodeToString(h[:])[:2]
}

// instanceHash returns the first 2 hex chars of SHA-256 of template + cat + microsecond timestamp.
func instanceHash(fullTemplate, cat string) string {
	us := fmt.Sprintf("%d", time.Now().UnixMicro())
	h := sha256.Sum256([]byte(fullTemplate + cat + us))
	return hex.EncodeToString(h[:])[:2]
}
```

Add `"crypto/sha256"` and `"encoding/hex"` to the imports.

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestWorkspaceHash -v`
Expected: PASS

- [ ] **Step 9: Add the cat name list and randomCatName**

Add to `internal/sandbox/naming.go`:

```go
var catNames = []string{
	"nyan", "maru", "chonk", "floof", "blep",
	"mlem", "loaf", "beans", "bongo", "mochi",
	"luna", "simba", "felix", "salem", "tom",
	"tux", "void", "smol", "purr", "meow",
	"socks", "fluff", "grump", "chomp", "boop",
}

// randomCatName picks a random cat name from the list.
func randomCatName() string {
	return catNames[rand.Intn(len(catNames))]
}
```

Add `"math/rand"` to the imports.

- [ ] **Step 10: Write a test for randomCatName**

Add to `internal/sandbox/naming_test.go`:

```go
func TestRandomCatName(t *testing.T) {
	name := randomCatName()
	if len(name) == 0 || len(name) > 5 {
		t.Errorf("randomCatName() = %q, want 1-5 chars", name)
	}
	found := false
	for _, c := range catNames {
		if c == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("randomCatName() = %q, not in catNames list", name)
	}
}
```

- [ ] **Step 11: Run all new tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run "TestTruncateClean|TestWorkspaceHash|TestRandomCatName" -v`
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add internal/sandbox/naming.go internal/sandbox/naming_test.go
git commit -m "feat: add naming helpers — truncateClean, workspaceHash, instanceHash, randomCatName"
```

---

### Task 2: Rewrite GenerateSandboxName and add WorkspacePrefix

**Files:**
- Modify: `internal/sandbox/naming.go`
- Modify: `internal/sandbox/naming_test.go`

- [ ] **Step 1: Write failing test for the new GenerateSandboxName format**

Replace the existing `TestGenerateSandboxName` in `internal/sandbox/naming_test.go`:

```go
func TestGenerateSandboxName(t *testing.T) {
	name := GenerateSandboxName("/path/to/my-project", "jvm")

	// Format: <wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
	pattern := `^[0-9a-f]{2}-[a-zA-Z0-9_.-]{1,12}\.\d{4}-[a-z]{1,5}-[0-9a-f]{2}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("GenerateSandboxName = %q, want match %s", name, pattern)
	}

	// Max 29 chars
	if len(name) > 29 {
		t.Errorf("GenerateSandboxName length = %d, want <= 29", len(name))
	}
}

func TestGenerateSandboxNameTruncatesLongWorkspace(t *testing.T) {
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", "jvm")

	// "lambda-jpm-clearings" is 20 chars, should truncate to 12
	// Workspace part is between first dash and the dot
	dotIdx := strings.Index(name, ".")
	if dotIdx == -1 {
		t.Fatalf("no dot in name: %q", name)
	}
	wsPart := name[:dotIdx] // e.g. "b4-lambda-jpm-c"
	// wshash is 2 chars + dash = 3, so workspace is wsPart[3:]
	ws := wsPart[3:]
	if len(ws) > 12 {
		t.Errorf("workspace portion %q exceeds 12 chars", ws)
	}
}

func TestGenerateSandboxNameTruncatesLongTemplate(t *testing.T) {
	name := GenerateSandboxName("/path/to/myapp", "kotlin-spring")

	// Template feeds into the hash but doesn't appear in the name directly
	// Just verify the name is valid and within length
	if len(name) > 29 {
		t.Errorf("name length = %d, want <= 29", len(name))
	}
}
```

Add `"strings"` to the test file imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run "TestGenerateSandboxName" -v`
Expected: FAIL — format doesn't match new pattern

- [ ] **Step 3: Rewrite GenerateSandboxName**

Replace the existing `GenerateSandboxName` in `internal/sandbox/naming.go`:

```go
// GenerateSandboxName returns: <wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
func GenerateSandboxName(workspacePath, template string) string {
	wsName := SanitizeWorkspaceName(filepath.Base(workspacePath))
	wsHash := workspaceHash(wsName)
	wsTrunc := truncateClean(wsName, 12)
	cat := randomCatName()
	iHash := instanceHash(template, cat)
	mmdd := time.Now().Format("0102")
	return fmt.Sprintf("%s-%s.%s-%s-%s", wsHash, wsTrunc, mmdd, cat, iHash)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run "TestGenerateSandboxName" -v`
Expected: PASS

- [ ] **Step 5: Write failing test for WorkspacePrefix**

Add to `internal/sandbox/naming_test.go`:

```go
func TestWorkspacePrefix(t *testing.T) {
	prefix := WorkspacePrefix("/path/to/lambda-jpm-clearings")

	// Should end with a dot
	if !strings.HasSuffix(prefix, ".") {
		t.Errorf("WorkspacePrefix = %q, want suffix '.'", prefix)
	}

	// Should match the beginning of a generated name for the same workspace
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", "jvm")
	if !strings.HasPrefix(name, prefix) {
		t.Errorf("name %q does not start with prefix %q", name, prefix)
	}

	// Different workspace with same 12-char prefix should get a different prefix
	prefix2 := WorkspacePrefix("/path/to/lambda-jpm-clients")
	if prefix == prefix2 {
		t.Errorf("different workspaces got same prefix: %q", prefix)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestWorkspacePrefix -v`
Expected: FAIL — `WorkspacePrefix` not defined

- [ ] **Step 7: Implement WorkspacePrefix**

Add to `internal/sandbox/naming.go`:

```go
// WorkspacePrefix returns the prefix used to match all sandboxes for a workspace.
// Format: <wshash(2)>-<workspace(12)>.
func WorkspacePrefix(workspacePath string) string {
	wsName := SanitizeWorkspaceName(filepath.Base(workspacePath))
	wsHash := workspaceHash(wsName)
	wsTrunc := truncateClean(wsName, 12)
	return fmt.Sprintf("%s-%s.", wsHash, wsTrunc)
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestWorkspacePrefix -v`
Expected: PASS

- [ ] **Step 9: Run all naming tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -v`
Expected: ALL PASS

- [ ] **Step 10: Commit**

```bash
git add internal/sandbox/naming.go internal/sandbox/naming_test.go
git commit -m "feat: rewrite GenerateSandboxName with length-bounded cat name format"
```

---

### Task 3: Update rm.go and resume.go prefix matching

**Files:**
- Modify: `internal/commands/rm.go:22-27`
- Modify: `internal/commands/resume.go:35-39`

- [ ] **Step 1: Update rm.go to use WorkspacePrefix**

In `internal/commands/rm.go`, replace line 27:

```go
// Old:
prefix := sandbox.SanitizeWorkspaceName(filepath.Base(wd)) + "-"
// New:
prefix := sandbox.WorkspacePrefix(wd)
```

Remove `"path/filepath"` from imports since it's no longer used. Keep `"os"` (still used for `os.Getwd()`).

- [ ] **Step 2: Update resume.go to use WorkspacePrefix**

In `internal/commands/resume.go`, replace line 39:

```go
// Old:
prefix := sandbox.SanitizeWorkspaceName(filepath.Base(wd)) + "-"
// New:
prefix := sandbox.WorkspacePrefix(wd)
```

Remove `"path/filepath"` from imports since it's no longer used.

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/andywang/Repos/claudebox && go build ./...`
Expected: No errors

- [ ] **Step 4: Run all tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/commands/rm.go internal/commands/resume.go
git commit -m "fix: use WorkspacePrefix for rm all and resume prefix matching"
```

---

### Task 4: Verify max length constraint

**Files:**
- Modify: `internal/sandbox/naming_test.go`

- [ ] **Step 1: Add a length constraint test**

Add to `internal/sandbox/naming_test.go`:

```go
func TestGenerateSandboxNameMaxLength(t *testing.T) {
	// Worst case: 12-char workspace, 5-char cat name
	// Run many times to exercise different cat names
	for i := 0; i < 100; i++ {
		name := GenerateSandboxName("/path/to/abcdefghijklmnopqrstuvwxyz", "long-template-name")
		if len(name) > 29 {
			t.Errorf("iteration %d: name %q length = %d, want <= 29", i, name, len(name))
		}
	}
}

func TestGenerateSandboxNameSocketPathFits(t *testing.T) {
	// Simulate a long home directory: /Users/christopherjohnson (25 chars)
	homeDir := "/Users/christopherjohnson"
	name := GenerateSandboxName("/path/to/some-long-workspace-name", "jvm")

	socketPath := homeDir + "/.docker/sandboxes/vm/" + name + "/docker-public.sock"
	if len(socketPath) > 103 {
		t.Errorf("socket path length = %d, want <= 103: %s", len(socketPath), socketPath)
	}
}
```

- [ ] **Step 2: Run the tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run "TestGenerateSandboxNameMaxLength|TestGenerateSandboxNameSocketPathFits" -v`
Expected: PASS

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/sandbox/naming_test.go
git commit -m "test: add max length and socket path constraint tests"
```
