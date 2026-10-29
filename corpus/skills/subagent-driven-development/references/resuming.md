<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/resuming.md -->

# Resuming From an Existing Ledger

The plan's private workspace already contains a ledger.

If the ledger's first line identifies this plan file, tasks with a
`Task <N>: complete` line are done. Do not dispatch them again. Resume at the
first task without that line. When the last line for a task records a fix
round, resume the loop at the next round. A ledger that identifies another
plan belongs to that plan. Leave it in place and start a fresh ledger for the
current plan.
