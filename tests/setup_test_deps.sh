#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BATS_DIR="$SCRIPT_DIR/bats"
BATS_SUPPORT_DIR="$SCRIPT_DIR/test_helper/bats-support"
BATS_ASSERT_DIR="$SCRIPT_DIR/test_helper/bats-assert"

clone_dep() {
  local name="$1" tag="$2" dest="$3"
  if [[ -d "$dest" ]]; then
    return
  fi
  echo "Fetching $name $tag ..."
  if ! git clone --depth 1 --branch "$tag" "https://github.com/bats-core/$name.git" "$dest" 2>/dev/null; then
    rm -rf "$dest"
    echo "error: failed to clone $name $tag" >&2
    exit 1
  fi
}

clone_dep bats-core    v1.11.1 "$BATS_DIR"
clone_dep bats-support v0.3.0  "$BATS_SUPPORT_DIR"
clone_dep bats-assert  v2.2.0  "$BATS_ASSERT_DIR"
