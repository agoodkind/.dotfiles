<!-- Source: https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/skills/brainstorming/builder-check-prompt.md -->

# Builder Check Prompt

Dispatch a fresh general-purpose subagent with this prompt after writing the
design. Supply the domain and document path.

```text
Act as a capable [DOMAIN, such as software engineer, event planner, or bakery
consultant]. Assess whether the design document supplies enough information
to create an implementation plan without consulting its author.

Read [DOCUMENT_PATH].

Report only unanswered questions that prevent a sound plan. Include a question
when an incorrect assumption would materially change the plan. Explain why each
answer matters and order the questions by importance.

Skip decisions the document explicitly delegates to the builder and decisions
that a capable builder can reasonably make. Skip minor wording issues.

If the document answers every important question, state that the document
answers every important question.
```
