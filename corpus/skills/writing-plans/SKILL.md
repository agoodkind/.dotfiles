---
name: writing-plans
description: Use with a specification or requirements for a task with several steps, before changing code.
---

<!-- Source: https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/skills/writing-plans/SKILL.md -->

# Writing Plans

## Overview

Write implementation plans for an engineer who has not seen the codebase or
specification. Assume the engineer can write idiomatic code once the plan
supplies the exact interface and verification. Document decisions the engineer
cannot infer, including files, names, signatures, specification values, and
tests that prove the required behavior. Allow reasonable implementation choices
where the plan does not constrain them.

Divide the work into small, coherent tasks. Avoid duplicated instructions and
unnecessary features. Commit frequently.

Follow the testing rule when deciding whether a task needs a new automated
test. Use direct verification when a new test is unnecessary.

Identify writing-plans as the active skill at the start.

Save plans under `docs/superpowers/plans/YYYY-MM-DD-<feature-name>.md`.
Follow the user's preferred location when the user supplies one.

## Check the Scope

Split a specification covering independent subsystems into separate plans
when brainstorming has not already divided the project. Each plan should
produce working software that supports independent verification.

## Define the File Structure

Before defining tasks, map each file that the plan will create or modify and
state its responsibility. Use that structure to divide the work into tasks.

- Give each unit a clear purpose and a well-defined interface.
- Prefer small, focused files that an engineer can understand in context.
- Keep files that change together close together. Split by responsibility
  rather than by technical layer.
- Follow established patterns in existing codebases. Do not restructure the
  codebase unilaterally. Include a targeted split when a file under modification
  has become difficult to maintain.

Each task should produce coherent changes that make sense independently.

## Size the Tasks

Use the smallest task that produces a coherent result for review. Include
setup, configuration, scaffolding, and documentation in the task that needs
them. Split tasks when a reviewer could reject one result while accepting
the adjacent result. End each task with an independently verifiable result.

## Define the Steps

Give each step one action and a checkable result. Examples include:

- Implement a function with the specified interface.
- Add a regression test when the testing rule requires one.
- Run the verification command and check its result.
- Commit the task.

## Use the Plan Header

Start every plan with this structure:

```markdown
# [Feature Name] Implementation Plan

> Use `subagent-driven-development` or `executing-plans` to implement the plan task by task. Track steps with checkbox syntax (`- [ ]`).

**Goal:** [State the observable result in one sentence.]

**Architecture:** [Explain the approach in 2-3 sentences.]

**Tech Stack:** [Specify the key technologies and libraries.]

**Spec:** [Link the specification or design document. Executors read both
the specification and the implementation plan.]

## Global Constraints

[State project-wide requirements, including version floors, dependency limits,
naming rules, copy requirements, and platform requirements. Use one line per
requirement and copy exact values from the specification. Every task includes
these constraints.]

## Review Focus

[Identify the five most consequential input classes or failure conditions
that the specification implies but the planned verification does not exercise.
State each input or condition and the expected behavior on one line, with the
most likely failure first. A specification does not enumerate every possible
input. Do not treat an unspecified input as permission for the program to fail.
Add the required test or direct verification to the task that implements the
behavior.]

---
```

## Use the Task Structure

Order implementation and any required test steps according to the task's needs.
The example below does not prescribe their order.

````markdown
### Task N: [Component Name]

**Files:**
- Create: `exact/path/to/file.py`
- Modify: `exact/path/to/existing.py:123-145`
- Test: `tests/exact/path/to/test.py` when the testing rule requires a test.

**Interfaces:**
- Consumes: [Specify the exact interfaces from earlier tasks.]
- Produces: [Specify exact function names, parameter types, and return types
  that later tasks use. An implementer may receive only this task.]

- [ ] **Step 1: Implement `function(input: InputType) -> ResultType` in `exact/path/to/file.py`**

[State the approach in one sentence when the signature and assertions leave
a material choice, such as a library call or data structure. Include an
implementation code block only when they do not determine the algorithm.]

- [ ] **Step 2: Add a test when the testing rule requires one**

Use a public entry point with real dependencies. Specify realistic input, the
production path, and a result visible to a user, caller, or downstream system.

```python
def test_specific_behavior() -> None:
    result = public_api.function(input)
    assert result == expected
```

- [ ] **Step 3: Verify the task**

Run: `pytest tests/path/test.py::test_name -v` when the task requires a test.
Otherwise, specify the direct verification command.
Expected: PASS or the specified observable result.

- [ ] **Step 4: Commit**

Stage only files that the task creates or modifies.

```bash
git add tests/path/test.py src/path/file.py
git commit -m "Add specific behavior in function"
```
````

## Supply the Decisions Each Step Needs

A step must let the implementer write one reasonable result. Include the
decisions that make the step unambiguous without writing unnecessary code.

- Give a test step its test name and assertions as code. Use the specification's
  exact values.
- Give an implementation step its exact signature, including name, parameters,
  and return type. Specify the file and any required values. Let the implementer
  write the body unless the signature and tests do not determine the algorithm,
  or the specification requires exact copy.
- Give a verification step its command and the output that proves success.
- Use the referenced task's Interfaces section when one task depends on another.
  Do not repeat the other task's implementation.

Record the decisions the implementer cannot make independently. A plan longer
than its implementation may contain unnecessary implementation code. Remove
instructions that do not decide anything, such as "TBD," "handle edge cases,"
"add appropriate validation," or "write tests for the above." Define every type
and function that a task references.

## Review the Complete Plan

Compare the complete plan with the specification. Run these checks directly
without dispatching a subagent:

1. Check specification coverage. Map each section and requirement to an
   implementation task. Add a task for any uncovered requirement.
2. Check each step. Supply missing decisions and remove implementation bodies
   that the signatures and tests already determine.
3. Check type consistency across tasks. Match method signatures, property names,
   and types to their earlier definitions. For example, correct a later use of
   `clearFullLayers()` when an earlier task defines `clearLayers()`.
4. Check Review Focus. Identify important input classes and failure conditions
   that the planned verification does not exercise. Record the five most
   consequential cases and add the required verification to the relevant tasks.
   Leave the section empty only after the check does not find any uncovered cases.
5. Check proportion. Compare the plan's length with the specification. If code
   blocks dominate the plan, replace unnecessary bodies with signatures, test
   names, and assertions. Confirm that each step remains unambiguous.

Fix issues directly. Complete one review pass rather than repeatedly reviewing
the same plan.

## Execution Handoff

<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/writing-plans/SKILL.md#execution-handoff -->

After saving and self-reviewing the plan, explain its design, task scope, and
material decisions in chat. Include the information needed for approval in
the conversation and provide the saved plan as an optional reference.
If the user has already explicitly supplied an execution method, ask
the user to review the plan and confirm it captures the intended result; wait for that
review before implementation, then use the preserved method. Otherwise, ask
the user to review the plan and choose an execution method before implementation.

When the user has not already supplied an execution method, use this prompt:

> [Present the proposed work in chat and include an optional link to the saved plan.] Which execution approach should the agent use?

- Subagent-driven execution uses a fresh subagent to implement each task and a fresh reviewer to check it before the next one starts, then a whole-branch review at the end. This method costs a fresh context per task and per review.
- Native execution uses the session agent to implement every task in this session, then one fresh reviewer on the most capable model checks the whole branch. This method costs one implementation context and one final reviewer context. Independent review occurs at the end. A mid-tier session model can use the plan's design decisions to implement the tasks.

> For this plan I recommend <one of the two>, because <one sentence from the plan: how much the tasks depend on each other's interfaces, how many there are, what a shipped mistake would cost>. Does the plan capture the intended result, and which approach should the agent use?

When the user has already supplied an execution method, use this prompt:

> [Present the proposed work in chat and include an optional link to the saved plan.] Does it capture the intended result?

If the user chooses subagent-driven execution, use
[subagent-driven-development](../subagent-driven-development/SKILL.md).

If the user chooses native execution, use
[executing-plans](../executing-plans/SKILL.md).
