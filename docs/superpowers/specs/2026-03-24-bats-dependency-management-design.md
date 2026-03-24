# Bats Dependency Management

## Overview

Standardize how bats-core, bats-assert, and bats-support are managed. Replace the current mixed approach (brew-installed bats-core + manually vendored helpers) with a fetch-on-demand setup script and a Makefile.

## Goals

- All three bats dependencies managed the same way
- Zero system-level dependencies beyond git and a POSIX shell
- One command to run tests: `make test`
- Deps fetched automatically on first run, cached locally

## Directory Structure

```
tests/
    bats/                          # bats-core (fetched, gitignored)
    test_helper/
        bats-support/              # fetched, gitignored
        bats-assert/               # fetched, gitignored
        common.bash                # existing shared test setup
    *.bats                         # existing test files
Makefile                           # new
tests/setup_test_deps.sh           # new
```

## Setup Script (`tests/setup_test_deps.sh`)

- Checks if each dependency directory exists; clones if missing
- Uses `git clone --depth 1 --branch <tag>` for each:
  - `bats-core` v1.11.1 -> `tests/bats/`
  - `bats-support` v0.3.0 -> `tests/test_helper/bats-support/`
  - `bats-assert` v2.2.0 -> `tests/test_helper/bats-assert/`
- Idempotent — running twice is a no-op
- No system-level installs required

## Makefile

```makefile
.PHONY: test setup-test-deps

test: setup-test-deps
	./tests/bats/bin/bats tests/*.bats

setup-test-deps:
	@./tests/setup_test_deps.sh
```

## Cleanup

- Delete currently vendored `tests/test_helper/bats-assert/` and `tests/test_helper/bats-support/`
- Update `.gitignore`: add `tests/bats/` entry (the two existing `bats-assert` and `bats-support` entries stay)
- No changes to `.bats` files — existing `load` paths remain valid
- Update README to document `make test`

## Version Pinning

Tags are pinned in `setup_test_deps.sh`. To update a dependency, bump the tag and delete the local directory. These libraries are mature and rarely change.
