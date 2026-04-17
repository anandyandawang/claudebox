package environment

import (
	"claudebox/internal/docker"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readGitIdentityFn returns the host's --global user.name and user.email.
// Empty strings on error or unset. Replaceable for testing.
var readGitIdentityFn = readGitIdentity

func readGitIdentity() (name, email string) {
	return readGitConfigGlobal("user.name"), readGitConfigGlobal("user.email")
}

func readGitConfigGlobal(key string) string {
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Setup configures the sandbox environment:
// exports GITHUB_USERNAME, configures JVM proxy, imports CA cert.
// Must only be called on create (fresh container); appends to /etc/sandbox-persistent.sh.
func Setup(d docker.Docker, sandboxName string) error {
	// Export GITHUB_USERNAME if set (GITHUB_TOKEN is auto-injected by `docker sandbox run`)
	if username := os.Getenv("GITHUB_USERNAME"); username != "" {
		script := fmt.Sprintf("printf 'export GITHUB_USERNAME=%%s\\n' %q >> /etc/sandbox-persistent.sh", username)
		if _, err := d.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
			return fmt.Errorf("setting GITHUB_USERNAME: %w", err)
		}
	}

	// Import host git identity into sandbox's global gitconfig.
	// Silently skip either value if unset on host.
	gitName, gitEmail := readGitIdentityFn()
	if gitName != "" {
		if _, err := d.SandboxExec(sandboxName, "git", "config", "--global", "user.name", gitName); err != nil {
			return fmt.Errorf("setting git user.name: %w", err)
		}
	}
	if gitEmail != "" {
		if _, err := d.SandboxExec(sandboxName, "git", "config", "--global", "user.email", gitEmail); err != nil {
			return fmt.Errorf("setting git user.email: %w", err)
		}
	}

	// Configure JVM proxy and import CA cert
	jvmScript := `if [ -n "$HTTPS_PROXY" ]; then
  PROXY_HOST=$(echo "$HTTPS_PROXY" | sed -E "s|https?://||;s|:.*||")
  PROXY_PORT=$(echo "$HTTPS_PROXY" | sed -E "s|.*:([0-9]+).*|\1|")
  echo "export JAVA_TOOL_OPTIONS=\"-Dhttp.proxyHost=${PROXY_HOST} -Dhttp.proxyPort=${PROXY_PORT} -Dhttps.proxyHost=${PROXY_HOST} -Dhttps.proxyPort=${PROXY_PORT}\"" >> /etc/sandbox-persistent.sh
fi
JAVA_HOME=$(java -XshowSettings:properties 2>&1 | grep "java.home" | awk "{print \$3}")
PROXY_CERT=$(find /usr/local/share/ca-certificates -name "*.crt" 2>/dev/null | head -1)
if [ -n "$PROXY_CERT" ] && [ -n "$JAVA_HOME" ]; then
  sudo keytool -import -trustcacerts -cacerts -storepass changeit -noprompt -alias proxy-ca -file "$PROXY_CERT" 2>/dev/null || true
fi`
	if _, err := d.SandboxExec(sandboxName, "sh", "-c", jvmScript); err != nil {
		return fmt.Errorf("configuring JVM proxy: %w", err)
	}
	return nil
}
