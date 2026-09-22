---
name: brainstorming
description: Use when a request is open-ended and material product, behavior, or architecture choices remain unresolved.
---

# Brainstorming

Turn an open-ended request into a clear implementation direction.

1. Inspect the current project and the user's stated constraints.
2. Identify the intended outcome, users, success criteria, and boundaries.
3. Ask only for missing choices that would materially change the result.
4. Present two or three approaches when a real tradeoff exists. State the recommended approach and its costs.
5. Describe the selected behavior, interfaces, error handling, and verification at the level needed for implementation.
6. Record a design document only when the user requests one, repository conventions require one, or the design is too large to remain clear in the task context.

Proceed once the request and available evidence determine an implementation direction. A planning-only request ends with the design or recommendation instead of implementation.

## Existing codebases

Follow established patterns after reading the relevant implementation. Include only refactors required by the requested behavior. Split the work when independent subsystems would otherwise create one oversized change.

## Visual comparisons

Use [visual-companion.md](visual-companion.md) when the user asks for visual exploration or accepts a visual comparison that materially improves a layout, mockup, or architecture decision.
