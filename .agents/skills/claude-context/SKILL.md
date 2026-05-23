---
name: claude-context
description: Use the claude-context MCP for semantic code discovery when the user explicitly asks for claude-context or semantic search. Sanity-check index quality first, prefer code-only extension filters for implementation questions, and fall back when results are docs or generated files.
disable-model-invocation: true
---

# Claude Context Search

Read the companion rule at [claude-context.mdc](../../rules/claude-context.mdc) before using this skill so the explicit workflow and the always-applied guidance stay aligned.

Use claude-context for conceptual discovery across a repo. Use native `grep`, `ripgrep`, or direct file reads for exact literals, exact line references, exhaustive occurrence lists, or when semantic results stay low-signal after one corrective pass.

## Workflow

1. Call `get_indexing_status` on the absolute repository root before searching.
2. If `get_indexing_status` returns "not indexed", stop immediately, ask the user whether to index the codebase, and wait for an explicit yes before calling `index_codebase`. Do not index without permission.
3. If indexing is still in progress, treat results as provisional and wait for `completed` before trusting ranking.
4. Look at the reported file and chunk counts. If the index is roughly one chunk per file, treat it as coarse and expect noisy whole-file matches.
5. Phrase the query as natural-language intent.
6. For implementation questions, start with `extensionFilter` for real source files such as `.go`, `.swift`, `.ts`, or `.py`. Do not search docs first unless the user is asking about docs.
7. Read the first results skeptically. Hits in `README.md`, `AGENTS.md`, `*.pb.go`, `*.grpc.pb.go`, or other generated files are warning signs, not answers.
8. If results are coarse, retry once with a tighter `extensionFilter` and a higher `limit`.
9. If results are still dominated by docs or generated files, stop using semantic search for that question and switch to native search or direct reads.
10. Reindex only when needed and only with user permission. Use `splitter: "ast"` by default. Use `splitter: "langchain"` only as a user-approved diagnostic, since it can still produce one-chunk-per-file indexes or reduce coverage.
11. Treat returned chunks as candidates. Read the cited files before acting because the working tree can be newer than the index.

## Subagent Instructions

When launching Task subagents, include this guidance verbatim in the prompt after substituting the absolute repository root for `<path>`:

> Use the claude-context MCP `search_code` tool first for conceptual code discovery in `<path>`. Call `get_indexing_status` before searching. If the status is "not indexed", stop, ask the user for permission to index, and wait for an explicit yes before calling `index_codebase`. Do not index without permission. Do not trust rankings while indexing is still in progress. If the reported stats are roughly one chunk per file, treat the index as coarse. For implementation questions, start with `extensionFilter` for the source language. If the top hits are docs like `README.md` or generated files like `*.pb.go`, retry once with tighter filters and a higher `limit`, then fall back to native `grep` or `ripgrep` instead of repeatedly forcing semantic search. Use `splitter: "ast"` by default when reindexing, and use `splitter: "langchain"` only when the user explicitly wants that diagnostic.

## Tool Reference

- `search_code(path, query, limit?, extensionFilter?)` returns ranked chunks.
- `get_indexing_status(path)` reports status and file or chunk counts.
- `index_codebase(path, force?, splitter?)` rebuilds the index.
- `clear_index(path)` removes the index and should only be used when the user explicitly wants a wipe or a splitter swap.
