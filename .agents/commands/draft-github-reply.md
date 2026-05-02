# Draft GitHub Reply

Use the current conversation context to draft a reply for GitHub (PR comment, issue comment, review). Output ONLY the reply in a single outer four-backtick fence so it stays copy-pasteable.

## Critical Rules

- **GitHub Flavored Markdown**: Use GFM syntax (headings, tables, task lists, syntax-highlighted code blocks, etc.)
- **Zero preamble/postamble**: The ENTIRE assistant message must be the reply itself. No "Here's your reply:", no "Let me draft that", no summary after. Nothing before, nothing after.
- **Copy-pasteable**: The reply should be directly pasteable into GitHub with correct rendering.
- **Protect copy-paste output from chat rendering**: Cursor renders chat as markdown and can mangle GitHub content before the user copies it. Wrap the entire reply in a single outer four-backtick fence so the content remains literal, even when the reply itself contains triple-backtick code blocks.
- **Inline code for symbols**: Use single backticks for code symbols, function names, variable names, file names, class names, reserved keywords, CLI flags, etc. (e.g. `updateTransactionDetails`, `isFromMerchantContext`, `--verbose`).

## GitHub Flavored Markdown Features (use freely)

- Headings: `#`, `##`, `###`
- Bold: `**bold**`
- Italic: `*italic*`
- Strikethrough: `~~struck~~`
- Inline code: use single backticks around items like `code`
- Code blocks with syntax highlighting: use fenced code blocks and include a language like `ruby`
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
3. Wrap the entire reply in a single outer four-backtick fence, with no language tag.
4. Output ONLY the reply. No other text in the message.

## Output Format

Always wrap the entire reply in a single outer four-backtick fenced block, with no language tag.

Do this whether or not the reply itself contains triple-backtick code blocks. The outer four-backtick fence avoids nesting conflicts and preserves the exact text for copy-paste into GitHub.

The assistant message must contain the reply and absolutely nothing else outside the outer four-backtick fence.
