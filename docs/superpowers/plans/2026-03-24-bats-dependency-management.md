# Bats Dependency Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace mixed bats dependency management (brew + manual clones) with a fetch-on-demand setup script and Makefile.

**Architecture:** A setup script clones pinned versions of bats-core, bats-support, and bats-assert on demand. A Makefile provides `make test` (fetch + run) and `make clean` (remove fetched deps). Existing test load paths remain unchanged.

**Tech Stack:** bash, make, git, bats-core

**Spec:** `docs/superpowers/specs/2026-03-24-bats-dependency-management-design.md`

---

### Task 1: Create the setup script

**Files:**
- Create: `tests/setup_test_deps.sh`

- [ ] **Step 1: Write the setup script**

```bash
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
```

- [ ] **Step 2: Make the script executable**

Run: `chmod +x tests/setup_test_deps.sh`

- [ ] **Step 3: Commit**

```bash
git add tests/setup_test_deps.sh
git commit -m "feat: add setup script to fetch bats test dependencies"
```

---

### Task 2: Create the Makefile

**Files:**
- Create: `Makefile`

- [ ] **Step 1: Write the Makefile**

```makefile
.PHONY: test setup-test-deps clean

test: setup-test-deps
	./tests/bats/bin/bats tests/*.bats

setup-test-deps:
	@./tests/setup_test_deps.sh

clean:
	rm -rf tests/bats tests/test_helper/bats-support tests/test_helper/bats-assert
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "feat: add Makefile with test and clean targets"
```

---

### Task 3: Update .gitignore

**Files:**
- Modify: `.gitignore:25-27`

- [ ] **Step 1: Add `tests/bats/` entry alongside existing bats entries**

Change the end of `.gitignore` from:

```
.env.local
tests/test_helper/bats-assert/
tests/test_helper/bats-support/
```

To:

```
.env.local

# Test dependencies (fetched by tests/setup_test_deps.sh)
tests/bats/
tests/test_helper/bats-assert/
tests/test_helper/bats-support/
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add tests/bats/ to gitignore"
```

---

### Task 4: Remove old vendored copies and verify

**Files:**
- Delete: `tests/test_helper/bats-assert/` (local only, not tracked)
- Delete: `tests/test_helper/bats-support/` (local only, not tracked)

- [ ] **Step 1: Delete old local copies**

Run: `rm -rf tests/test_helper/bats-assert tests/test_helper/bats-support`

- [ ] **Step 2: Run `make test` to fetch fresh deps and run tests**

Run: `make test`

Expected: Setup script fetches all three deps, then bats runs all tests and they pass.

- [ ] **Step 3: Run `make test` again to verify idempotency**

Run: `make test`

Expected: No "Fetching..." output (deps already present), tests pass.

- [ ] **Step 4: Run `make clean` then `make test` to verify clean cycle**

Run: `make clean && make test`

Expected: Deps removed, then re-fetched, tests pass.

---

### Task 5: Update README

**Files:**
- Modify: `README.md` (append after line 76, the end of "How it works")

- [ ] **Step 1: Add Development section to README**

After the "How it works" section (end of file), add:

```markdown
## Development

### Running tests

```bash
make test
```

This fetches test dependencies (bats-core, bats-assert, bats-support) on first run, then executes the test suite. The only prerequisite is git.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add development section to README with make test instructions"
```
