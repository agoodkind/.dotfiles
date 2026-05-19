---
name: claude-context
description: Use the claude-context MCP for semantic code search instead of native grep, ripgrep, or find, and instruct subagents to do the same. Use when the user explicitly asks for claude-context, semantic code search, or code-context, or mentions this skill by name.
---

# Claude Context Search

Prefer claude-context's semantic `search_code` over native `grep`, `ripgrep`, `find`, or read-then-scan workflows whenever the query is conceptual or discovery-oriented. Carry the same preference into every subagent launched during this task.

## Companion Rule

The persistent guidance for when to reach for claude-context versus native search lives in [claude-context.mdc](../../rules/claude-context.mdc). Read that rule before using this skill so the heuristics for "use claude-context" versus "use grep" stay in sync between this workflow and the always-applied rule.

## When to Use claude-context

- Conceptual or discovery queries: "where is X", "how does Y work", "what handles Z", "find the code that does W".
- Unfamiliar codebases or wide cross-cutting questions.
- Any time the answer depends on intent rather than a literal string match.

## When to Stay With grep

- Exact literal matches (an error message, a precise symbol, a regex with anchors).
- Need every occurrence, not a ranked top-k.
- Need exact line numbers and surrounding context.
- Single file or small scope where reading directly is faster.
- claude-context returned nothing useful and you have a literal fallback.

## Workflow

1. Confirm the codebase is indexed before searching. Call `get_indexing_status` against the absolute repository root.
2. If status is anything other than `completed`, call `index_codebase` with `path` set to the absolute repo root and `splitter: "ast"`. Poll `get_indexing_status` until `completed`. Skip this step if the index is already fresh.
3. Phrase the query as a natural-language description of intent, not as a regex or symbol name.
4. Call `search_code` with the absolute `path` and the natural-language `query`. Use `extensionFilter` when the search is restricted to specific file types. Keep `limit` at the default unless the top results miss the answer.
5. Treat results as ranked candidates. Open the cited file regions with the Read tool before acting on them, since indexed chunks can lag the working tree.
6. Fall back to native `grep` or `ripgrep` only for literal-string matches or when claude-context returns nothing useful.

## Subagent Instructions

When launching Task subagents (explore, generalPurpose, or shell), include this guidance verbatim in the subagent prompt:

> Use the claude-context MCP `search_code` tool against the absolute repository path `<path>` for any code discovery query. Phrase queries as natural-language intent. Fall back to native `grep` or `ripgrep` only for literal-string matches or when `search_code` returns nothing useful. Confirm the codebase is indexed via `get_indexing_status` before searching, and call `index_codebase` with `splitter: "ast"` if it is not.

Always substitute `<path>` with the absolute repository root before sending the prompt so the subagent does not have to infer it.

## Tool Reference

- `search_code(path, query, limit?, extensionFilter?)` returns ranked code chunks with file paths and rough line ranges.
- `index_codebase(path, force?, splitter?)` builds or rebuilds the index. Use `splitter: "ast"` for code, `"langchain"` only when AST splitting is unavailable for the file type.
- `get_indexing_status(path)` reports the current index state, percentage progress when actively indexing, and the last completion timestamp.
- `clear_index(path)` removes the index. Only call when the user explicitly asks for a wipe, or when an index has become corrupted.
