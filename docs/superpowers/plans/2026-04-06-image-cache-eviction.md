# Image Cache Eviction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically prune stale Docker sandbox image-cache tars on every claudebox invocation to prevent unbounded disk growth.

**Architecture:** A standalone `PruneImageCache()` function in a new `internal/cache` package, called once at startup in `main.go` before cobra dispatches. It reads `~/.docker/sandboxes/image-cache/`, deletes `.tar` files older than 1 hour, skips `.tmp-*` files, and logs warnings on errors without blocking the command.

**Tech Stack:** Go stdlib only (`os`, `path/filepath`, `time`, `fmt`)

---

### File Structure

- Create: `internal/cache/prune.go` — `PruneImageCache()` function
- Create: `internal/cache/prune_test.go` — unit tests
- Modify: `cmd/claudebox/main.go` — call `PruneImageCache()` at startup

---

### Task 1: Write `PruneImageCache` with tests

**Files:**
- Create: `internal/cache/prune.go`
- Create: `internal/cache/prune_test.go`

- [ ] **Step 1: Write the failing test for basic eviction**

Create `internal/cache/prune_test.go`:

```go
// internal/cache/prune_test.go
package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneImageCache_DeletesStaleTars(t *testing.T) {
	dir := t.TempDir()
	staleFile := filepath.Join(dir, "jvm-sandbox-7fc06f01-abc123.tar")
	if err := os.WriteFile(staleFile, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Backdate modification time to 2 hours ago
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(staleFile, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	PruneImageCache(dir)

	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("expected stale tar to be deleted, but it still exists")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/cache/ -run TestPruneImageCache_DeletesStaleTars -v`

Expected: FAIL — package `cache` does not exist yet.

- [ ] **Step 3: Write minimal implementation**

Create `internal/cache/prune.go`:

```go
// internal/cache/prune.go
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxAge = 1 * time.Hour

// PruneImageCache deletes .tar files older than 1 hour from the given
// image-cache directory. Skips .tmp-* files (active downloads). Errors
// are logged to stderr but never fatal.
func PruneImageCache(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // directory missing or unreadable — nothing to do
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".tar") || strings.HasPrefix(name, ".tmp-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to prune %s: %v\n", name, err)
			}
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/cache/ -run TestPruneImageCache_DeletesStaleTars -v`

Expected: PASS

- [ ] **Step 5: Write test that fresh tars are preserved**

Add to `internal/cache/prune_test.go`:

```go
func TestPruneImageCache_PreservesFreshTars(t *testing.T) {
	dir := t.TempDir()
	freshFile := filepath.Join(dir, "jvm-sandbox-7fc06f01-def456.tar")
	if err := os.WriteFile(freshFile, []byte("fresh"), 0o600); err != nil {
		t.Fatal(err)
	}
	// ModTime is now — well within the 1-hour window

	PruneImageCache(dir)

	if _, err := os.Stat(freshFile); err != nil {
		t.Errorf("expected fresh tar to be preserved, but got: %v", err)
	}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/cache/ -run TestPruneImageCache_PreservesFreshTars -v`

Expected: PASS

- [ ] **Step 7: Write test that .tmp- files are skipped**

Add to `internal/cache/prune_test.go`:

```go
func TestPruneImageCache_SkipsTmpFiles(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, ".tmp-jvm-sandbox-7fc06f01-aaa.tar1234567")
	if err := os.WriteFile(tmpFile, []byte("downloading"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(tmpFile, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	PruneImageCache(dir)

	if _, err := os.Stat(tmpFile); err != nil {
		t.Errorf("expected .tmp- file to be preserved, but got: %v", err)
	}
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/cache/ -run TestPruneImageCache_SkipsTmpFiles -v`

Expected: PASS

- [ ] **Step 9: Write test that non-.tar files are skipped**

Add to `internal/cache/prune_test.go`:

```go
func TestPruneImageCache_SkipsNonTarFiles(t *testing.T) {
	dir := t.TempDir()
	otherFile := filepath.Join(dir, "daemon.log")
	if err := os.WriteFile(otherFile, []byte("logs"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(otherFile, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	PruneImageCache(dir)

	if _, err := os.Stat(otherFile); err != nil {
		t.Errorf("expected non-tar file to be preserved, but got: %v", err)
	}
}
```

- [ ] **Step 10: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/cache/ -run TestPruneImageCache_SkipsNonTarFiles -v`

Expected: PASS

- [ ] **Step 11: Write test that missing directory is a no-op**

Add to `internal/cache/prune_test.go`:

```go
func TestPruneImageCache_MissingDirIsNoop(t *testing.T) {
	// Should not panic or error — just return silently
	PruneImageCache("/nonexistent/path/that/does/not/exist")
}
```

- [ ] **Step 12: Run all cache tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/cache/ -v`

Expected: All 5 tests PASS

- [ ] **Step 13: Commit**

```bash
git add internal/cache/prune.go internal/cache/prune_test.go
git commit -m "feat: add image cache eviction for stale sandbox tars"
```

---

### Task 2: Wire `PruneImageCache` into `main.go`

**Files:**
- Modify: `cmd/claudebox/main.go:14` (inside `main()`, before `rootCmd`)

- [ ] **Step 1: Add the PruneImageCache call to main.go**

Add the import and call after `d := docker.NewClient()` and before the `rootCmd` definition in `cmd/claudebox/main.go`:

```go
import (
	"claudebox/internal/cache"
	"claudebox/internal/commands"
	"claudebox/internal/docker"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func main() {
	templatesDir, err := findTemplatesDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	d := docker.NewClient()

	// Prune stale image-cache tars before running any command.
	imageCacheDir := filepath.Join(os.Getenv("HOME"), ".docker", "sandboxes", "image-cache")
	cache.PruneImageCache(imageCacheDir)

	rootCmd := &cobra.Command{
```

- [ ] **Step 2: Run all tests to verify nothing broke**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`

Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/claudebox/main.go
git commit -m "feat: wire image cache eviction into claudebox startup"
```

---

### Task 3: Manual smoke test

- [ ] **Step 1: Verify eviction works on real image cache**

Run: `cd /Users/andywang/Repos/claudebox && ls -lT ~/.docker/sandboxes/image-cache/`

Note which tars are older than 1 hour.

- [ ] **Step 2: Run any claudebox command**

Run: `cd /Users/andywang/Repos/claudebox && go run ./cmd/claudebox ls`

- [ ] **Step 3: Verify stale tars were deleted**

Run: `ls -lT ~/.docker/sandboxes/image-cache/`

Expected: Tars older than 1 hour are gone. Fresh tars and `.tmp-*` files remain.

- [ ] **Step 4: Commit the spec**

```bash
git add docs/superpowers/specs/2026-04-06-image-cache-eviction-design.md docs/superpowers/plans/2026-04-06-image-cache-eviction.md
git commit -m "docs: add image cache eviction design spec and plan"
```
