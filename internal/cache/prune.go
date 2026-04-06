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
