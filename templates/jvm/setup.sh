#!/bin/sh
# JVM proxy and CA cert configuration.
# Runs inside the sandbox at create time.

# Configure JVM proxy if HTTPS_PROXY is set (injected by Docker Desktop)
if [ -n "$HTTPS_PROXY" ]; then
  PROXY_HOST=$(echo "$HTTPS_PROXY" | sed -E "s|https?://||;s|:.*||")
  PROXY_PORT=$(echo "$HTTPS_PROXY" | sed -E "s|.*:([0-9]+).*|\1|")
  echo "export JAVA_TOOL_OPTIONS=\"-Dhttp.proxyHost=${PROXY_HOST} -Dhttp.proxyPort=${PROXY_PORT} -Dhttps.proxyHost=${PROXY_HOST} -Dhttps.proxyPort=${PROXY_PORT}\"" >> /etc/sandbox-persistent.sh
fi

# Import proxy CA cert into Java truststore
JAVA_HOME=$(java -XshowSettings:properties 2>&1 | grep "java.home" | awk '{print $3}')
PROXY_CERT=$(find /usr/local/share/ca-certificates -name "*.crt" 2>/dev/null | head -1)
if [ -n "$PROXY_CERT" ] && [ -n "$JAVA_HOME" ]; then
  sudo keytool -import -trustcacerts -cacerts -storepass changeit -noprompt -alias proxy-ca -file "$PROXY_CERT" 2>/dev/null || true
fi
