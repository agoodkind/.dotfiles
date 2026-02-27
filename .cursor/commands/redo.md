# Redo

The last output broke a rule. Regenerate it correctly.

## Steps

1. Identify which rule(s) the last output violated. The user may specify the rule (e.g. `/redo voice-tone`, `/redo audience-grounding`). If no rule is named, infer from context which rule(s) were broken.
2. Re-read the relevant rule file(s) from `.cursor/rules/`.
3. Regenerate the last artifact (PR description, commit message, doc, Slack message, code comment, or whatever the last output was) with the rule applied correctly.

## Critical Rules

- Do not apologize, explain what went wrong, or describe what you're doing differently. Just produce the corrected output.
- Do not ask clarifying questions. If the violated rule is ambiguous, apply the most reasonable interpretation and output the result.
- The corrected output replaces the previous one entirely. Do not produce a diff or partial fix.
