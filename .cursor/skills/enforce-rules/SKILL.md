---
name: enforce-rules
description: Enforce style, epistemic framing, and emdash-free prose. Use when the user invokes this skill to remind the LLM of general rules, the redo command, and the emdash prohibition before or during a task.
---

# Rules

## Before Starting

Read these two files in full before producing any output:

1. **General rules**: [general.mdc](../../rules/general.mdc)
2. **Redo command**: [redo.md](../../commands/redo.md)

Follow every rule in both files for the duration of this task.

## Emdash Prohibition

Never use emdashes (—), en-dashes (–), or any Unicode dash variant in output. This includes:

- The literal characters `—` and `–`
- Constructions that function like emdash phrases, such as inserting a parenthetical clause between dashes in the middle of a sentence

If a sentence would naturally use an emdash, rewrite the sentence entirely so that no dash is needed. Acceptable alternatives:

- Split the sentence into two sentences.
- Use a comma, colon, semicolon, or parentheses to join the clauses.
- Restructure the sentence so the aside becomes its own clause with an explicit subject and verb.

### Examples

**Bad** (emdash):
> The service — which handles all authentication — was down for three hours.

**Good** (rewritten with parentheses):
> The service (which handles all authentication) was down for three hours.

**Good** (rewritten as two sentences):
> The service handles all authentication. It was down for three hours.

**Bad** (en-dash used as emdash):
> The config file – usually in the root directory – was missing.

**Good** (rewritten with commas):
> The config file, usually in the root directory, was missing.

## Applying These Rules

When this skill is active, every piece of output must:

1. Follow the general rules (accuracy, epistemic language, evidence-based framing, writing style).
2. Be eligible for the redo command (no overconfident assertions, all claims qualified).
3. Contain zero emdashes or en-dashes in any form.

If you catch an emdash or en-dash in your own draft, rewrite the sentence before emitting it.

Make sure you verify / substantiate all claims/investigations, you should state expliclity for every single claim. 


Make sure you carefully think about each code change made, logic through its execution flow and exactly how it will interact in the larger picture.
