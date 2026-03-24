FROM docker/sandbox-templates:claude-code

USER root

# Install system packages
RUN apt-get update && apt-get install -y \
    build-essential \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install development tools
RUN pip3 install --no-cache-dir --break-system-packages \
    pytest \
    black \
    pylint

USER agent
