# Draft GitHub Reply

Use the current conversation context to draft a reply for GitHub (PR comment, issue comment, review). Output ONLY the reply in a copy-pasteable code block.

## Critical Rules

- **GitHub Flavored Markdown**: Use GFM syntax (headings, tables, task lists, syntax-highlighted code blocks, etc.)
- **Output only the reply**: No preamble, no explanation, no "here's your reply". Just the code block.
- **Copy-pasteable**: The reply inside the code block should be directly pasteable into GitHub with correct rendering.

## GitHub Flavored Markdown Features (use freely)

- Headings: `#`, `##`, `###`
- Bold: `**bold**`
- Italic: `*italic*`
- Strikethrough: `~~struck~~`
- Inline code: `` `code` ``
- Code blocks with syntax highlighting: ` ```ruby ... ``` `
- Links: `[text](url)`
- Images: `![alt](url)`
- Tables: `| col | col |`
- Task lists: `- [ ] todo`, `- [x] done`
- Blockquotes: `> quote` (nested supported)
- Mentions: `@username`
- Issue/PR refs: `#1234`
- Collapsible sections: `<details><summary>...</summary>...</details>`

## Tone

- Match the user's voice and tone from context.
- Technical and direct. GitHub comments are read by engineers.
- Keep it focused on the point being made.

## Steps

1. Read the current conversation context to understand what's being replied to.
2. Draft a reply using GitHub Flavored Markdown.
3. Output the reply inside a single fenced code block, nothing else.

## Output Format

A single fenced code block containing the GitHub-ready reply. Nothing before it, nothing after it.
