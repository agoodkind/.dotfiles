<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/batching.md -->

# Batching Small Same-Shape Work

When the plan lists several small, independent edits of the same kind, such as
the same one-line fix, constant change, or field addition across files, do not
dispatch one subagent per task. Compose one brief listing every file and its
change. Send the batch to one subagent and review its diff as one unit. Use a
separate dispatch for work that needs its own judgment, tests, or review.
