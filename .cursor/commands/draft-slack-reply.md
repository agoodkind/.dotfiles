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

Do NOT backtick:
- General technical concepts ("the API", "the database", "caching")
- Product/service names ("Redis", "Postgres", "Kubernetes") unless referring to a CLI command or config value
- Plain English descriptions of behavior

## Writing Style

- **No emdashes**: Never use emdashes (—) or emdash-like constructs (--). Restructure sentences to use periods, commas, colons, or parentheses instead.
- **No filler words**: Avoid "improves", "enhances", "streamlines", "ensures", "allows", "enables" when used as justification. State facts directly.
- **No sycophancy or AI-isms**: No "delve", "tapestry", "I'd be happy to", "Great question!". Write like an engineer, not a chatbot.
- **Concise**: No fluff or preambles. Get to the point.
- **Direct**: Present tense, direct facts. State what is, not what "should be noted".
- Match the user's voice and tone from context.
- Keep it conversational. Slack is not email.
- Use thread-appropriate formality (match the context).

## Steps

1. Read the current conversation context to understand what's being replied to.
2. Draft a reply using Slack mrkdwn formatting.
3. Output the reply directly as your entire response. No code fences, no preamble, no explanation.

## Output Format

The raw Slack message text, output directly. Your entire response IS the Slack message. Do not wrap it in anything.
