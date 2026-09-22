---
name: writing-plans
description: Use when a multi-step implementation needs an explicit sequence, file map, interfaces, and verification instructions.
---

# Writing Plans

Write an implementation plan that another engineer can execute without rediscovering the design.

Use exact paths, symbols, commands, dependencies, and expected outcomes. Follow existing repository structure and conventions. Split independent subsystems into separate plans when they can be implemented and reviewed independently.

## Plan structure

```markdown
# <Feature> implementation plan

## Goal
<Observable result>

## Current behavior
<Verified starting state>

## Constraints
<Exact project-wide requirements>

## Tasks

### 1. <Task name>

Files:
- Modify: `path/to/file`
- Create: `path/to/file`

Behavior:
- <Required behavior and interfaces>

Steps:
1. <Concrete implementation step>
2. <Concrete implementation step>

Verification:
- Run: `<command or direct inspection>`
- Expect: `<observable result>`
```

Each task should create one coherent result. State task dependencies and interface contracts explicitly. Include code only when exact code is required to prevent ambiguity.

Add a new test step only when the testing rule requires a new test. When testing is required, specify the public boundary, realistic input, production path, and observable assertion. Otherwise specify the direct command or inspection that proves the task.

Remove placeholders, repeated instructions, speculative work, and unrelated cleanup. Save the plan only when the user requests a file or the repository already uses plan documents. Otherwise return it in the current conversation.
