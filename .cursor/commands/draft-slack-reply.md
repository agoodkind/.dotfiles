# Draft Slack Reply

Use the current conversation context to draft a reply for Slack. Output ONLY the reply text directly in chat (no wrapper).

## Critical Rules

- **Slack mrkdwn, NOT Markdown**: Slack uses its own `mrkdwn` format. It is NOT standard or GitHub-flavored markdown.
- **Output ONLY the reply**: No preamble, no explanation, no "here's your reply", no code fences. Just output the raw Slack message text directly. The user will copy it from the chat.

## Slack mrkdwn Syntax (use these, not standard markdown)

| Element        | Slack mrkdwn          | Standard Markdown (DO NOT USE) |
|----------------|-----------------------|-------------------------------|
| Bold           | `*bold*`              | `**bold**`                    |
| Italic         | `_italic_`            | `*italic*`                    |
| Strikethrough  | `~struck~`            | `~~struck~~`                  |
| Inline code    | `` `code` ``          | `` `code` ``                  |
| Code block     | ` ``` `               | ` ```lang `                   |
| Link           | plain URL only (see below) | `[text](url)`            |
| Bulleted list  | `• item` or `- item`  | `- item`                      |
| Blockquote     | `> quote`             | `> quote`                     |
| User mention   | `<@U12345>`           | N/A                           |
| Emoji          | `:emoji_name:`        | N/A                           |

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

Use backticks for:
- Function/method names: `` `handleSubmit` ``, `` `fetchUser()` ``
- Variable/constant names: `` `userId` ``, `` `MAX_RETRIES` ``
- Class/module names: `` `UserService` ``, `` `ApplicationController` ``
- File paths: `` `src/utils/api.ts` ``
- CLI commands: `` `git rebase` ``, `` `bundle exec rspec` ``
- Config keys, env vars: `` `RAILS_ENV` ``, `` `database.yml` ``
- HTTP methods/status codes: `` `POST` ``, `` `404` ``
- Boolean/keyword values: `` `true` ``, `` `nil` ``, `` `undefined` ``
- Error names/types: `` `TypeError` ``, `` `ActiveRecord::RecordNotFound` ``
- Gem/package names: `` `sidekiq` ``, `` `lodash` ``

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
3. Output the reply directly as your entire response. No code fences, no preamble, no explanation.

## Output Format

The raw Slack message text, output directly. Your entire response IS the Slack message. Do not wrap it in anything.
