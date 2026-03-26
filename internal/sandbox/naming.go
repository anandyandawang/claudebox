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

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_.\-]+`)

// truncateClean truncates s to max chars and strips trailing hyphens and dots.
func truncateClean(s string, max int) string {
	if len(s) > max {
		s = s[:max]
	}
	return strings.TrimRight(s, "-.")
}

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

// SanitizeWorkspaceName replaces non-alphanumeric chars (except _ . -) with hyphens.
// Matches the Bash: tr -cs 'a-zA-Z0-9_.-' '-'
func SanitizeWorkspaceName(name string) string {
	return nonAlphanumeric.ReplaceAllString(name, "-")
}

// GenerateSessionID returns a session ID: sandbox-YYYYMMDD-HHMMSS.
func GenerateSessionID() string {
	return fmt.Sprintf("sandbox-%s", time.Now().Format("20060102-150405"))
}

// WorkspacePrefix returns the prefix used to match all sandboxes for a workspace.
// Format: <wshash(2)>-<workspace(12)>.
func WorkspacePrefix(workspacePath string) string {
	wsName := SanitizeWorkspaceName(filepath.Base(workspacePath))
	wsHash := workspaceHash(wsName)
	wsTrunc := truncateClean(wsName, 12)
	return fmt.Sprintf("%s-%s.", wsHash, wsTrunc)
}

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
