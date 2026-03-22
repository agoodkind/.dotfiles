# Remove Emdashes

When the user types "remove emdashes", the last response likely contains emdashes (—), en-dashes (–), or sentences structured around dash-based parentheticals.

## Why LLMs Produce These Patterns

LLMs reach for emdash constructions for a specific underlying reason: they want to pack a qualification, context, or aside into the same sentence as the main claim, without committing to a separate sentence. The result is a sentence shaped like "X [pause] important detail [pause] continues." This shape appears even when no literal emdash is used. It shows up as a spaced hyphen, a double hyphen, a long parenthetical, or a clause that trails off and restarts. The fix is never to swap the punctuation; it is to decompose the sentence so the aside becomes its own sentence or clause with an explicit subject and verb.

## What to Catch

Flag and rewrite any sentence that:

- Contains a literal emdash (`—`), en-dash (`–`), or any Unicode dash variant (‒, ―, ‑, or similar).
- Uses two hyphens (`--`) as a stand-in for an emdash.
- Uses a single hyphen (` - `) surrounded by spaces to insert a parenthetical or aside into the middle of a sentence, functioning the way an emdash would.
- Is structurally shaped like an emdash construction: a main clause, a dash-separated aside or appositive, then a continuation, even if the character used is not technically an emdash.
- Inserts a qualification, caveat, or context parenthetical mid-sentence in a way that interrupts the flow and then resumes the main clause, regardless of the punctuation used.

If you are unsure whether a sentence is structured like an emdash, rewrite it anyway.

## Steps

1. Scan the last response for any of the patterns described above, including structural patterns even when no dash character is present.
2. For each flagged sentence, ask: "Why did the writer insert this aside here instead of making it its own sentence?" Then restructure the sentence to answer that question directly.
3. Acceptable rewrites: split into two sentences, use commas, use colons, use semicolons, use parentheses, or restructure the clause so the aside becomes its own sentence with an explicit subject and verb.
4. Reproduce the full corrected response with all flagged sentences rewritten.

## Critical Rules

- Produce the corrected output only. No apology, no explanation, no description of what changed.
- Every flagged sentence must be fully rewritten, not patched.
- The corrected output must contain zero emdashes, en-dashes, hyphen-space constructions used as emdashes, double-hyphens used as emdashes, or any Unicode dash variant (—, –, ‒, ―, ‑).
- Do not fix the punctuation and leave the structure intact. Fix the structure.
- Preserve the meaning and tone of the original sentence.
- Do not alter sentences that had no dashes and were not structured like emdash constructions.
