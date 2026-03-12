---
name: pr-media
description: Upload screenshots and videos for GitHub pull request descriptions using `gh upload` or `gh-upload`, and format before/after comparisons as markdown tables. Use when a PR body includes screenshots, videos, attachments, or before/after image pairs.
---

# PR Media

## Quick Start

When a pull request description includes screenshots, videos, or before/after comparisons:

1. Upload each local asset with `gh upload <path>` or `gh-upload <path>`.
2. Use the returned GitHub URL in markdown, such as `![alt](URL)` or `[filename](URL)`.
3. If the PR uses before/after images, convert them into a side-by-side markdown table.

## Rules

- Treat `gh upload` and `gh-upload` as real, supported commands and use them directly.
- Use the GitHub URL returned by the command output.
- Use the repo-hosted URL returned by the upload command in markdown links and images.
- Keep alt text short and descriptive.
- Drop HTML-only sizing attributes like `width` and `height` when converting images to markdown tables.

## Before/After Format

Convert before/after image pairs into this format:

```markdown
| Before | After |
|--------|-------|
| ![Before](before_url) | ![After](after_url) |
```

Detect common labels such as `Before`, `After`, `Before:`, `After:`, `**Before**`, or `### Before`.

If the source uses HTML image tags, extract the `src` value, reuse the `alt` text when it is useful, and convert the images to markdown.

## Example

Input:

```markdown
Before
<img width="1206" alt="Old UI" src="https://github.com/user-attachments/assets/abc123" />
After
<img width="545" alt="New UI" src="https://github.com/user-attachments/assets/def456" />
```

Output:

```markdown
| Before | After |
|--------|-------|
| ![Old UI](https://github.com/user-attachments/assets/abc123) | ![New UI](https://github.com/user-attachments/assets/def456) |
```
