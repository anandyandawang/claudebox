# Repo Restructuring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move tool source into `src/` and templates into `templates/` while keeping `claudebox` at the repo root as the entry point.

**Architecture:** `git mv` files into `src/` and `templates/`, then update all path references in the dispatcher, command files, test files, and README. No functional changes.

**Tech Stack:** Bash, BATS (testing)

---

## File Structure

**Moved files:**
- `commands/` → `src/commands/`
- `lib/` → `src/lib/`
- `jvm/` → `templates/jvm/`
- `python/` → `templates/python/`

**Modified files (path updates):**
- `claudebox` (dispatcher)
- `src/commands/create.sh` (template resolution)
- `tests/claudebox.bats`
- `tests/create.bats`
- `tests/helpers.bats`
- `tests/ls.bats`
- `tests/resume.bats`
- `tests/rm.bats`
- `tests/test_helper/common.bash`
- `README.md`

---

### Task 1: Move files with git mv

**Files:**
- Move: `commands/` → `src/commands/`
- Move: `lib/` → `src/lib/`
- Move: `jvm/` → `templates/jvm/`
- Move: `python/` → `templates/python/`

- [ ] **Step 1: Create target directories and move files**

```bash
mkdir -p src templates
git mv commands src/commands
git mv lib src/lib
git mv jvm templates/jvm
git mv python templates/python
```

- [ ] **Step 2: Commit the move**

```bash
git add -A
git commit -m "refactor: move commands/lib to src/ and templates to templates/"
```

---

### Task 2: Update claudebox dispatcher

**Files:**
- Modify: `claudebox:13` (helpers source path)
- Modify: `claudebox:34` (usage template glob)
- Modify: `claudebox:46,51,56,61` (command source paths)

- [ ] **Step 1: Update helpers source path**

Change line 13 from:
```bash
source "${SCRIPT_DIR}/lib/helpers.sh"
```
to:
```bash
source "${SCRIPT_DIR}/src/lib/helpers.sh"
```

- [ ] **Step 2: Update usage() template glob**

Change line 34 from:
```bash
  for dir in "${SCRIPT_DIR}"/*/; do
```
to:
```bash
  for dir in "${SCRIPT_DIR}/templates"/*/; do
```

- [ ] **Step 3: Update command source paths**

Change all four command sources (lines 46, 51, 56, 61) from `"${SCRIPT_DIR}/commands/..."` to `"${SCRIPT_DIR}/src/commands/..."`:

```bash
# Line 46
source "${SCRIPT_DIR}/src/commands/ls.sh"
# Line 51
source "${SCRIPT_DIR}/src/commands/rm.sh"
# Line 56
source "${SCRIPT_DIR}/src/commands/resume.sh"
# Line 61
source "${SCRIPT_DIR}/src/commands/create.sh"
```

- [ ] **Step 4: Commit**

```bash
git add claudebox
git commit -m "refactor: update claudebox dispatcher paths for src/ and templates/"
```

---

### Task 3: Update create.sh template resolution

**Files:**
- Modify: `src/commands/create.sh:7` (TEMPLATE_DIR)

- [ ] **Step 1: Update template path**

Change line 7 from:
```bash
  TEMPLATE_DIR="${SCRIPT_DIR}/${TEMPLATE}"
```
to:
```bash
  TEMPLATE_DIR="${SCRIPT_DIR}/templates/${TEMPLATE}"
```

- [ ] **Step 2: Commit**

```bash
git add src/commands/create.sh
git commit -m "refactor: update create.sh template path to templates/"
```

---

### Task 4: Update test files

**Files:**
- Modify: `tests/helpers.bats:29` (source path)
- Modify: `tests/ls.bats:31-33` (source paths)
- Modify: `tests/resume.bats:39-41` (source paths)
- Modify: `tests/rm.bats:30-31` (source paths)
- Modify: `tests/create.bats:37-39` (source paths)
- Modify: `tests/create.bats:86` (fake template directory structure)
- Modify: `tests/claudebox.bats` (no source path changes — it invokes the real script)

- [ ] **Step 1: Update tests/helpers.bats source path**

Change line 29 from:
```bash
  source "${SCRIPT_DIR}/lib/helpers.sh"
```
to:
```bash
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
```

Also update the shellcheck directive on line 28 from `../lib/helpers.sh` to `../src/lib/helpers.sh`.

- [ ] **Step 2: Update tests/ls.bats source paths**

Change lines 31-33 from:
```bash
  source "${SCRIPT_DIR}/lib/helpers.sh"
  # shellcheck source=../commands/ls.sh
  source "${SCRIPT_DIR}/commands/ls.sh"
```
to:
```bash
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../src/commands/ls.sh
  source "${SCRIPT_DIR}/src/commands/ls.sh"
```

Also update the shellcheck directive on line 30 from `../lib/helpers.sh` to `../src/lib/helpers.sh`.

- [ ] **Step 3: Update tests/resume.bats source paths**

Change lines 38-41 from:
```bash
  # shellcheck source=../lib/helpers.sh
  source "${SCRIPT_DIR}/lib/helpers.sh"
  # shellcheck source=../commands/resume.sh
  source "${SCRIPT_DIR}/commands/resume.sh"
```
to:
```bash
  # shellcheck source=../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../src/commands/resume.sh
  source "${SCRIPT_DIR}/src/commands/resume.sh"
```

Also update the inline `source` calls inside **all six** `run bash -c '...'` blocks (lines 113-114, 126-127, 148-149, 168-169, 187-188, 205-206) from:
```bash
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
```
to:
```bash
    source "${SCRIPT_DIR}/src/lib/helpers.sh"
    source "${SCRIPT_DIR}/src/commands/resume.sh"
```

- [ ] **Step 4: Update tests/rm.bats source paths**

Change lines 30-31 from:
```bash
  source "${SCRIPT_DIR}/lib/helpers.sh"
  source "${SCRIPT_DIR}/commands/rm.sh"
```
to:
```bash
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  source "${SCRIPT_DIR}/src/commands/rm.sh"
```

- [ ] **Step 5: Update tests/create.bats source paths and fake template helper**

Change lines 36-39 from:
```bash
  # shellcheck source=../lib/helpers.sh
  source "${SCRIPT_DIR}/lib/helpers.sh"
  # shellcheck source=../commands/create.sh
  source "${SCRIPT_DIR}/commands/create.sh"
```
to:
```bash
  # shellcheck source=../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../src/commands/create.sh
  source "${SCRIPT_DIR}/src/commands/create.sh"
```

Update the `setup_fake_template()` helper (line 86) from:
```bash
  mkdir -p "${fake_root}/${template_name}"
  touch "${fake_root}/${template_name}/Dockerfile"
```
to:
```bash
  mkdir -p "${fake_root}/templates/${template_name}"
  touch "${fake_root}/templates/${template_name}/Dockerfile"
```

Also update the test that creates `allowed-hosts.txt` directly (lines 267 and 314). The `template_dir` variable on line 267 currently reads:
```bash
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/mytemplate"
```
Change to:
```bash
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/templates/mytemplate"
```

And on line 314:
```bash
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/mytemplate"
```
Change to:
```bash
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/templates/mytemplate"
```

- [ ] **Step 6: Run tests**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add tests/
git commit -m "refactor: update test paths for src/ and templates/ layout"
```

---

### Task 5: Update README

**Files:**
- Modify: `README.md:47` (template description)
- Modify: `README.md:56-63` (template creation section)

- [ ] **Step 1: Update template description**

Change line 47 from:
```
Each subdirectory with a `Dockerfile` is a template. Built-in templates:
```
to:
```
Each subdirectory under `templates/` with a `Dockerfile` is a template. Built-in templates:
```

- [ ] **Step 2: Update template creation section**

Change lines 59-63 from:
```
my-template/
  Dockerfile
  allowed-hosts.txt   # optional
```
to:
```
templates/
  my-template/
    Dockerfile
    allowed-hosts.txt   # optional
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update README paths for new repo layout"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 2: Verify usage output lists templates**

```bash
./claudebox 2>&1 | head -20
```

Expected: `Available templates:` section lists `jvm` and `python`.

- [ ] **Step 3: Verify directory structure**

```bash
ls -la src/commands/ src/lib/ templates/jvm/ templates/python/
```

Expected: all files present in new locations.
