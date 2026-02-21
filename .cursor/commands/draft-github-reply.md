# Draft GitHub Reply

Use the current conversation context to draft a reply for GitHub (PR comment, issue comment, review). Output ONLY the reply, copy-pasteable.

## Critical Rules

- **GitHub Flavored Markdown**: Use GFM syntax (headings, tables, task lists, syntax-highlighted code blocks, etc.)
- **Zero preamble/postamble**: The ENTIRE assistant message must be the reply itself. No "Here's your reply:", no "Let me draft that", no summary after. Nothing before, nothing after.
- **Copy-pasteable**: The reply should be directly pasteable into GitHub with correct rendering.
- **No nested fences**: If the reply contains fenced code blocks, do NOT wrap the entire reply in an outer code fence. The inner triple backticks close the outer fence, breaking the output.
- **Inline code for symbols**: Use single backticks for code symbols, function names, variable names, file names, class names, reserved keywords, CLI flags, etc. (e.g. `updateTransactionDetails`, `isFromMerchantContext`, `--verbose`).

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

## Writing Style

- **No emdashes**: Never use emdashes (—) or emdash-like constructs (--). Restructure sentences to use periods, commas, colons, or parentheses instead.
- **No filler words**: Avoid "improves", "enhances", "streamlines", "ensures", "allows", "enables" when used as justification. State facts directly.
- **No sycophancy or AI-isms**: No "delve", "tapestry", "I'd be happy to", "Great question!". Write like an engineer, not a chatbot.
- **Concise**: No fluff or preambles. Get to the point.
- **Direct**: Present tense, direct facts. State what is, not what "should be noted".
- Match the user's voice and tone from context.
- Technical and direct. GitHub comments are read by engineers.
- Keep it focused on the point being made.

## Steps

1. Read the current conversation context to understand what's being replied to.
2. Draft a reply using GitHub Flavored Markdown.
3. Check if the reply contains fenced code blocks (triple backticks).
4. Output ONLY the reply. No other text in the message.

## Output Format

- **Reply has NO code blocks**: Wrap the entire reply in a single fenced code block.
- **Reply HAS code blocks**: Output the reply as plain text (no outer fence). Nesting fenced code blocks inside a fenced code block breaks rendering. The inner triple backticks close the outer fence prematurely, cutting off the rest of the reply.

The assistant message must contain the reply and absolutely nothing else.
