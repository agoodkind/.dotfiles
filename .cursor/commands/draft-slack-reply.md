# Draft Slack Reply

Use the current conversation context to draft a reply for Slack. Output ONLY the reply text in a single outer four-backtick fence so it stays copy-pasteable in chat.

## Critical Rules

- **Slack mrkdwn, NOT Markdown**: Slack uses its own `mrkdwn` format. It is NOT standard or GitHub-flavored markdown.
- **Protect copy-paste output from chat rendering**: Cursor renders chat as markdown and can mangle Slack content that contains `*bold*`, pipes, brackets, tables, or code fences. Wrap the entire reply in a single outer four-backtick fence so the content stays literal and copy-pasteable.
- **Output ONLY the reply content**: No preamble, no explanation, and no wrapper text outside the outer four-backtick fence. The user will copy the Slack message from inside the fence.

## Slack mrkdwn Syntax (use these, not standard markdown)

- Bold: Slack `*bold*`, not Markdown `**bold**`
- Italic: Slack `_italic_`, not Markdown `*italic*`
- Strikethrough: Slack `~struck~`, not Markdown `~~struck~~`
- Inline code: use single backticks around items like `code`
- Code block: use triple backticks with no language tag
- Link: plain URL only (see below), not `[text](url)`
- Bulleted list: `• item` or `- item`
- Blockquote: `> quote`
- User mention: `<@U12345>`
- Emoji: `:emoji_name:`

## Links

The `<url|text>` hyperlink syntax only works for messages sent programmatically via the Slack API. For messages the user will copy-paste into Slack manually, it renders as literal text. For copy-paste messages:

- Use plain URLs only: Slack will auto-link them
- Do NOT use `<url|text>` anchor text syntax
- If you need to reference an external page inline, name it in prose and put the URL on its own line or at the end

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
3. Output the reply as your entire response inside a single outer four-backtick fence, with no language tag and no text outside the fence.

## Output Format

Wrap the entire Slack reply in a single outer four-backtick fenced block, with no language tag.

Why: a four-backtick outer fence does not conflict with inner triple-backtick code blocks that may appear in the Slack message, and it prevents Cursor chat from rendering Slack mrkdwn as standard markdown.
