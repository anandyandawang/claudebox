package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxSandboxNameLen = 29
	maxWorkspaceLen   = 12
	catNameLen        = 5
	wsHashLen         = 2
	instanceHashLen   = 2
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_.\-]+`)

// hexHashPrefix returns the first n hex chars of the SHA-256 of input.
func hexHashPrefix(input string, n int) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:n]
}

// truncateClean truncates s to max chars and strips trailing hyphens and dots.
func truncateClean(s string, max int) string {
	if len(s) > max {
		s = s[:max]
	}
	return strings.TrimRight(s, "-.")
}

// workspaceHash returns the first 2 hex chars of SHA-256 of the full workspace path.
func workspaceHash(fullWorkspace string) string {
	return hexHashPrefix(fullWorkspace, wsHashLen)
}

// instanceHash returns the first 2 hex chars of SHA-256 of template + cat + microsecond timestamp.
func instanceHash(fullTemplate, cat string) string {
	return hexHashPrefix(fullTemplate+cat+fmt.Sprintf("%d", time.Now().UnixMicro()), instanceHashLen)
}

var catNames = []string{
	"chonk", "floof", "beans", "bongo", "mochi",
	"socks", "fluff", "grump", "chomp", "tabby",
	"catto", "meows", "purrs", "bonks", "bloop",
	"smols", "nyans", "marus", "bleps", "mlems",
	"loafs", "boops", "snoot", "toeby", "pawsy",
}

// randomCatName picks a random cat name from the list.
func randomCatName() string {
	return catNames[rand.Intn(len(catNames))]
}

// SanitizeWorkspaceName replaces non-alphanumeric chars (except _ . -) with hyphens.
// Matches the Bash: tr -cs 'a-zA-Z0-9_.-' '-'
func SanitizeWorkspaceName(name string) string {
	return nonAlphanumeric.ReplaceAllString(name, "-")
}

// GenerateSessionID returns a session ID: sandbox-YYYYMMDD-HHMMSS.
func GenerateSessionID() string {
	return fmt.Sprintf("sandbox-%s", time.Now().Format("20060102-150405"))
}

// sanitizedWorkspace returns the truncated workspace name, falling back to
// a hash prefix if the name is empty after sanitization and truncation.
func sanitizedWorkspace(workspacePath string) string {
	ws := truncateClean(SanitizeWorkspaceName(filepath.Base(workspacePath)), maxWorkspaceLen)
	if ws == "" {
		ws = hexHashPrefix(workspacePath, maxWorkspaceLen)
	}
	return ws
}

// WorkspacePrefix returns the prefix used to match all sandboxes for a workspace.
// Format: <wshash(2)>-<workspace(12)>.
func WorkspacePrefix(workspacePath string) string {
	wsHash := workspaceHash(workspacePath)
	wsTrunc := sanitizedWorkspace(workspacePath)
	return fmt.Sprintf("%s-%s.", wsHash, wsTrunc)
}

// GenerateSandboxName returns: <wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
func GenerateSandboxName(workspacePath, template string) string {
	prefix := WorkspacePrefix(workspacePath)
	cat := randomCatName()
	iHash := instanceHash(template, cat)
	mmdd := time.Now().Format("0102")
	return fmt.Sprintf("%s%s-%s-%s", prefix, mmdd, cat, iHash)
}
