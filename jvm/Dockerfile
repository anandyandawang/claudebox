FROM docker/sandbox-templates:claude-code

USER root

# Install prerequisites
RUN apt-get update && apt-get install -y wget gnupg && rm -rf /var/lib/apt/lists/*

# Add Adoptium apt repository (pinned to noble — questing not yet supported)
RUN mkdir -p /etc/apt/keyrings && \
    wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public | gpg --dearmor -o /etc/apt/keyrings/adoptium.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/adoptium.gpg] https://packages.adoptium.net/artifactory/deb noble main" > /etc/apt/sources.list.d/adoptium.list

# Install JDK 21 and dev tools
RUN apt-get update && apt-get install -y \
    temurin-21-jdk \
    fzf \
    jq \
    vim \
    nano \
    && rm -rf /var/lib/apt/lists/*

# Install git-delta for enhanced diffs
ARG GIT_DELTA_VERSION=0.18.2
RUN ARCH=$(dpkg --print-architecture) && \
    if [ "$ARCH" = "amd64" ]; then DELTA_ARCH="x86_64-unknown-linux-musl"; \
    elif [ "$ARCH" = "arm64" ]; then DELTA_ARCH="aarch64-unknown-linux-gnu"; \
    else echo "Unsupported arch: $ARCH" && exit 1; fi && \
    wget -q "https://github.com/dandavison/delta/releases/download/${GIT_DELTA_VERSION}/delta-${GIT_DELTA_VERSION}-${DELTA_ARCH}.tar.gz" && \
    tar -xzf "delta-${GIT_DELTA_VERSION}-${DELTA_ARCH}.tar.gz" && \
    mv "delta-${GIT_DELTA_VERSION}-${DELTA_ARCH}/delta" /usr/local/bin/ && \
    rm -rf "delta-${GIT_DELTA_VERSION}-${DELTA_ARCH}" "delta-${GIT_DELTA_VERSION}-${DELTA_ARCH}.tar.gz"

USER agent

# Disable Gradle daemon to prevent cache corruption from concurrent builds.
# Claude Code often runs multiple Gradle commands in parallel, and the daemon's
# file locks don't play well with the sandbox filesystem.
RUN mkdir -p /home/agent/.gradle && \
    printf "org.gradle.daemon=false\norg.gradle.parallel=false\n" > /home/agent/.gradle/gradle.properties
