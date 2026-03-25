package sandbox

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_.\-]+`)

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
