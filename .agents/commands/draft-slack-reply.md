# Draft Slack Reply

Use the current conversation context to draft a reply for Slack. Output ONLY the reply text with nothing else around it so the user can copy and paste it directly.

## Critical Rules

- **Slack mrkdwn, NOT Markdown**: Slack uses its own `mrkdwn` format. It is NOT standard or GitHub-flavored markdown.
- **Output ONLY the reply content**: No preamble, no explanation, no wrapper text, no fences, and nothing else outside the reply itself. The user will copy the entire response as the Slack message.

## Slack mrkdwn Syntax (use these, not standard markdown)

- Bold: Slack `*bold*`, not Markdown `**bold**`
- Italic: Slack `_italic_`, not Markdown `*italic*`
- Strikethrough: Slack `~struck~`, not Markdown `~~struck~~`
- Inline code: use single backticks around items like `code`
- Code block: use triple backticks with no language tag
- Link: use markdown `[descriptive text](url)` — renders as clickable blue text in Cursor chat, pastes cleanly into Slack
- Bulleted list: `• item` or `- item`
- Blockquote: `> quote`
- User mention: use `@Name` verbatim (e.g. `@Alex Goodkind`)
- Emoji: `:emoji_name:`

## Links

The output is rendered in Cursor chat as markdown before the user copies it. Use this to your advantage for links:

- Use standard markdown link syntax `[descriptive text](url)` so the link renders as clickable blue text in Cursor chat.
- When the user pastes into Slack, Slack will receive the plain text label and the URL will be auto-linked by Slack.
- Do NOT use plain bare URLs inline. Do NOT use `<url|text>` anchor syntax (that only works via the Slack API).
- The descriptive text should name the specific thing being linked (e.g. `[her March 11 message](url)`, `[Rona's question](url)`, `[ref](url)`) so the reader understands what they are clicking before clicking.

Example:
`From the thread, it sounded like [these were two separate things](https://chime.slack.com/archives/...).`

## NOT supported in Slack mrkdwn (never use these)

- No headings (`#`, `##`, etc.)
- No tables
- No syntax highlighting on code blocks (no language after triple backticks)
- No nested blockquotes
- No images or inline HTML
- No numbered lists with auto-incrementing (use manual numbers or bullets)

## Inline Code (Backticks)

Use inline code for technical references such as:

- Function/method names like `handleSubmit` and `fetchUser()`
- Variable/constant names like `userId` and `MAX_RETRIES`
- Class/module names like `UserService` and `ApplicationController`
- File paths like `src/utils/api.ts`
- CLI commands like `git rebase` and `bundle exec rspec`
- Config keys and env vars like `RAILS_ENV` and `database.yml`
- HTTP methods and status codes like `POST` and `404`
- Boolean and keyword values like `true`, `nil`, and `undefined`
- Error names and types like `TypeError` and `ActiveRecord::RecordNotFound`
- Gem and package names like `sidekiq` and `lodash`

## Writing Style

- **No emdashes**: Never use emdashes (—) or emdash-like constructs (--). Restructure or reflow the sentence as needed to avoid them.
- **No filler words**: Do not use words like "improves", "enhances", "streamlines", "ensures", "allows", or "enables" as justification for a change, since they add no information. State the actual fact instead.
- **No sycophancy**: Do not use phrases like "delve", "tapestry", "I'd be happy to", or "Great question!", because they are AI-isms that make the message sound unnatural and hollow.
- **Tone**: Write like a human talking to a colleague, not like a bug report. Avoid openers that add no information, like "Just wanted to flag" or "Quick note:", where the first sentence could simply be the message itself.
- **No fragments**: Every sentence must have a subject and a verb, must begin with a capital letter, and must end with punctuation.
- **Be concise**: Do not add preamble or restate what the reader already knows, since that forces them to read more to get to the point.
- **Use present tense**: State what is happening, not what "should be noted" or "may be worth considering".
- Write conversationally and match the formality level of the thread or channel being replied to, since Slack is not email and formal constructions feel out of place.

## Steps

1. Read the current conversation context to understand what's being replied to.
2. Draft a reply using Slack mrkdwn formatting.
3. Output the reply as your entire response with nothing else around it. No fences, no preamble, no explanation.
