# Catvox Rename Design

**Date:** 2026-03-27
**Status:** Approved

## Summary

Rename the project from "claudebox" to "catvox" (cat + evolution + box). Update all references — repo name, CLI binary, Go module, imports, strings, docs. CLI commands stay the same. Add a README section explaining the name and the Schrödinger's cat analogy.

## Name Origin

**catvox** = **cat** + e**vo**lution + bo**x**

Inspired by Schrödinger's cat thought experiment, mapped to AI agents in sandboxes:

| Schrödinger's | Catvox |
|---|---|
| Cat | Filesystem / project repo |
| Poisonous gas | Claude running with `--dangerously-skip-permissions` |
| Box | Sandbox container |
| Human / observer | Real host filesystem |
| Cat dies | Agent goes rogue, destroys filesystem |
| Throw out box + cat | `rm` the sandbox, start fresh |
| Testing gas first | Running without `--dangerously` (permission prompts) |

## What Changes

### Binary and module
- CLI binary: `claudebox` → `catvox`
- Go module path: update `go.mod` module name
- All internal import paths referencing the module
- `cmd/claudebox/` directory → `cmd/catvox/`
- Makefile build targets and binary references
- `.gitignore` binary entry

### Strings and docs
- README.md: all `claudebox` references → `catvox`, plus new "What is catvox?" section
- Existing design specs and plans: only update CLI usage examples and forward-looking references. Historical references (e.g., "we renamed from claudebox") can stay as-is.
- CLAUDE.md if it references "claudebox"
- Docker image naming in code (e.g., `jvm-sandbox` stays — that's Docker-level, not project-level)

### README addition

New section at the top explaining the name:

> **catvox** = **cat** + e**vo**lution + bo**x**
>
> Inspired by Schrödinger's cat: you put a cat (your repo) in a box (a sandbox container) and let it evolve with Claude. 99% of the time the mutations are good — new features, refactors, fixes. But 1% of the time the cat dies (the agent goes rogue and the filesystem explodes). With catvox, the damage stays in the box. Throw it out, get a new box, get a new cat, start evolving again.
>
> Without the box, the gas hits *you*.

## What Stays the Same

- CLI commands: `catvox jvm`, `catvox ls`, `catvox resume`, `catvox rm`, `catvox rm all`
- Sandbox naming scheme (cat names: chonk, floof, beans, etc.)
- All internal architecture, code structure, packages
- Template system and Docker integration
- Network policy, credentials, environment setup

## Scope

This is a rename-only change. No behavioral changes, no new features, no architectural modifications.
