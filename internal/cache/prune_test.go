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
	// Backdate modification time to 2 * maxAge ago
	staleTime := time.Now().Add(-2 * maxAge)
	if err := os.Chtimes(staleFile, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	PruneImageCache(dir)

	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("expected stale tar to be deleted, but it still exists")
	}
}

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

func TestPruneImageCache_SkipsTmpFiles(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, ".tmp-jvm-sandbox-7fc06f01-aaa.tar")
	if err := os.WriteFile(tmpFile, []byte("downloading"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * maxAge)
	if err := os.Chtimes(tmpFile, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	PruneImageCache(dir)

	if _, err := os.Stat(tmpFile); err != nil {
		t.Errorf("expected .tmp- file to be preserved, but got: %v", err)
	}
}

func TestPruneImageCache_SkipsNonTarFiles(t *testing.T) {
	dir := t.TempDir()
	otherFile := filepath.Join(dir, "daemon.log")
	if err := os.WriteFile(otherFile, []byte("logs"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * maxAge)
	if err := os.Chtimes(otherFile, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	PruneImageCache(dir)

	if _, err := os.Stat(otherFile); err != nil {
		t.Errorf("expected non-tar file to be preserved, but got: %v", err)
	}
}

func TestPruneImageCache_MissingDirIsNoop(t *testing.T) {
	// Should not panic or error — just return silently
	PruneImageCache("/nonexistent/path/that/does/not/exist")
}
