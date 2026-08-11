# Skill Body Embedding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `SkillBody` embedding with shared rule and skill cycle detection.

**Architecture:** Extend skill rendering with one typed body expansion engine. The engine strips skill front matter, expands nested templates, and reports the active cycle path.

**Tech Stack:** Go, `text/template`, Go tests.

## Global Constraints

Keep reference rendering unchanged.

Use one implementation for rule and skill expansion.

Reject invalid input before writing rendered files.

---

### Task 1: Specify body expansion behavior

**Files:**
- Modify: [skills_test.go](../../../dots/internal/sync/compilation/skills_test.go)
- Modify: [source_set_render_test.go](../../../dots/internal/sync/compilation/source_set_render_test.go)

**Interfaces:**
- Consumes: `RenderSkillDirsFromSourceSet(CorpusSourceSet, string, SkillRefStyle) error`
- Produces: coverage for `{{.SkillBody "name"}}` and typed cycle errors

- [ ] Add a test that embeds a templated skill body and omits its front matter.
- [ ] Add direct skill, nested skill, rule, and cross-type cycle tests.
- [ ] Run `go test ./internal/sync/compilation` from `dots` and confirm the new tests fail for missing behavior.

### Task 2: Implement shared expansion

**Files:**
- Modify: [source_set_render.go](../../../dots/internal/sync/compilation/source_set_render.go)
- Modify: [compilation.go](../../../dots/internal/sync/compilation/compilation.go)

**Interfaces:**
- Consumes: `CorpusSourceSet.Rules`, `CorpusSourceSet.Skills`, and the root skill name
- Produces: `RuleBody(string) string`, `SkillBody(string) string`, and typed cycle errors

- [ ] Replace rule-only expansion state with typed expansion nodes and one body renderer.
- [ ] Extract the embedded `SKILL.md` or `SKILL.md.tmpl` body without front matter.
- [ ] Seed expansion with the root skill so recursive inclusion fails immediately.
- [ ] Run `go test ./internal/sync/compilation` from `dots` and confirm all focused tests pass.

### Task 3: Verify the compiler

**Files:**
- Modify only files required by failing checks.

**Interfaces:**
- Consumes: the completed compiler change
- Produces: formatted, tested repository state

- [ ] Run `gofmt` on modified Go files.
- [ ] Run `make check` from `dots`.
- [ ] Run the repository check target from the root when it differs from `dots/Makefile`.
- [ ] Review `git diff --check` and the complete diff.
- [ ] Commit the verified change with the required signed commit and trailer.
