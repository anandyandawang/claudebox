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

// GenerateSandboxName returns: <workspace>-<template>-sandbox-YYYYMMDD-HHMMSS.
func GenerateSandboxName(workspacePath, template string) string {
	wsName := SanitizeWorkspaceName(filepath.Base(workspacePath))
	return fmt.Sprintf("%s-%s-%s", wsName, template, GenerateSessionID())
}
