package environment

import (
	"claudebox/internal/docker"
	"fmt"
	"os"
)

// Setup configures the sandbox environment:
// exports GITHUB_USERNAME, configures JVM proxy, imports CA cert.
func Setup(d docker.Docker, sandboxName string) error {
	// Export GITHUB_USERNAME if set (GITHUB_TOKEN is auto-injected by `docker sandbox run`)
	if username := os.Getenv("GITHUB_USERNAME"); username != "" {
		script := fmt.Sprintf("printf 'export GITHUB_USERNAME=%%s\\n' %q >> /etc/sandbox-persistent.sh", username)
		if _, err := d.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
			return fmt.Errorf("setting GITHUB_USERNAME: %w", err)
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
