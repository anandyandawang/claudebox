package credentials

import (
	"claudebox/internal/docker"
	"claudebox/internal/sandbox"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readKeychainFn reads credentials from macOS Keychain. Replaceable for testing.
var readKeychainFn = readKeychain

func readKeychain() (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Refresh reads credentials from macOS Keychain and injects them into the sandbox.
func Refresh(d docker.Docker, sandboxName string) error {
	creds, err := readKeychainFn()
	if err != nil || creds == "" {
		if err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: No credentials found in Keychain. You may need to re-authenticate inside the sandbox.")
		}
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(creds))
	script := fmt.Sprintf(
		"echo '%s' | tr -d '[:space:]' | base64 -d > %s/.credentials.json && chmod 600 %s/.credentials.json",
		encoded, sandbox.SandboxClaudeDir, sandbox.SandboxClaudeDir)
	_, err = d.SandboxExec(sandboxName, "sh", "-c", script)
	return err
}
