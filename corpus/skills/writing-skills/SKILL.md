---
name: writing-skills
description: Use when creating or revising reusable agent skills and their supporting resources.
---

# Writing Skills

Write only the guidance that changes an agent's decisions or improves a repeated task.

## Scope

Create a skill for a reusable technique, workflow, or reference. Put repository-specific conventions in repository instructions. Prefer automation for mechanical constraints.

Use a lowercase hyphenated directory name. Each skill requires `SKILL.md` with concise `name` and `description` frontmatter. The description should identify the requests that activate the skill without summarizing its entire workflow.

## Content

State the outcome, triggers, boundaries, and non-obvious constraints. Preserve user choices and existing authorization limits. Use absolute language only for real safety or correctness invariants.

Keep the entrypoint short. Move conditional detail into linked references. Add scripts only when repeated deterministic execution improves reliability. Add examples only when they materially clarify application.

Use positive instructions for the desired output shape. Use conditional rules for behavior that depends on observable facts. Remove repeated checklists and speculative edge cases.

For additional authoring guidance, read [anthropic-best-practices.md](anthropic-best-practices.md). Use [graphviz-conventions.dot](graphviz-conventions.dot) and `node render-graphs.js <skill-directory>` only when a decision diagram materially improves the skill.

## Validation

Check frontmatter, file references, invocation wording, and rendered output. Run behavioral evaluation only when the skill's complexity or risk makes that evidence necessary. Validate changed scripts by running them.
