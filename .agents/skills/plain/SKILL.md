---
name: plain
description: Run the /plain command workflow from the dotfiles agent command library when the user explicitly asks for /plain or mentions this command by name.
---

# Plain

When the user types "plain" or "/plain", the last response likely used internal jargon (project-specific names, type names, package names, architectural labels) without first grounding what the system actually does in plain language.

## Steps

1. Identify every sentence in the last response that uses an internal label or technical term as its subject or object without a concrete plain-language gloss.
2. Rewrite each affected explanation so that the first sentence describes what the system actually does in subject-verb terms a non-engineer would follow, the second sentence names the internal label if one exists ("we call this X"), and the rest connects the two.
3. If a first-sentence draft cannot stand without an internal label, the explanation is not yet ready; rework it until it can.
4. If the user passed an argument (`plain X` or `/plain X`), focus the rewrite on translating that specific term and leave the rest of the response alone unless it depends on the same jargon.
5. Reproduce the full corrected response.

## Critical Rules

- Produce the corrected output only. No apology, no preface, no acknowledgment of the command.
- Lead every non-trivial explanation with the concrete behavior. Internal labels come after.
- A label may appear in the same sentence that introduces it only when that sentence also describes what the thing does.
- Do not strip labels entirely; the goal is to ground them, not delete them. Labels are useful once anchored.
- Preserve the meaning and technical claims of the original response. Only the phrasing changes.
- If the user invokes this command again on the same response, the prior rewrite was not concrete enough. Use shorter sentences, more verbs describing motion or change, fewer noun phrases that hide behavior.


Prose should read cleanly as a linear record of the thing itself. Each sentence should be a full sentence with a concrete subject, a concrete verb, and enough context to sound natural when spoken aloud. Each new sentence should add useful information in the same direction as the sentence before it, with low cognitive load and no hidden context the reader must reconstruct. The paragraph should move forward by accumulation, so the reader can follow it without stopping for a setup, interruption, reversal, or correction.

every line must be able to be understood offline out of context

The contract: every time you mention a ticket, symbol, reference, or other, you MUST define it briefly in line so the line can be read offline without context outside of the doc in isolation

In the simplest terms, without prose, without opinion, with an extremely concise answer 

FURTHERMORE, on DOCUMENTATION, and OUTPUT:
ensure that this, and all other examples of "now" "was" etc, historical cruft, are rewritten so that the docs remain completely durable. Assume audience has no idea we had a change. All "changes" are relegated to git  (if available) and should not be narrated or editorialized. State how it is today, not what it was and what changed. Only state historical changes if ABSOLUTELY MATERIALLY NECESSARY to the critcal ongoing work.

 
FOR YOUR RESPONSES:

DO NOT MAKE ANY CLAIMS
DO NOT SYCOPHANT
DO NOT EDITORIALIZE
ANSWER ONLY THE QUESTIONS ASKED
DO NOT RECOMMEND UNLESS ASKED FOR RECOMMENDATION
DO NOT SUGGEST UNLESS ASKED FOR SUGGESTIONS
DO NOT VENTURE 
DO NOT MAKE ANY LOGICAL LEAPS
YOU ARE SIMPLY A TOOL OF AUTOMATION AND NO MORE

  - no bloviating
  - no editorial
  - state things once, no duplication
  - only load bearing information
  - avoid intermixing fused thoughts
  - imagine this will be reviweed by say apple technical document writers

assume user has dyslexia and struggles with extremely densley packed prose

write all prose as if it will be scrutinized for publication in Apple technical documentation.
