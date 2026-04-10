#!/usr/bin/env python3
"""Auto-fix shell shorthand [[ ]]/if violations found by ast-grep.

Rewrites:
  [[ cond ]] && action   →  if [[ cond ]]; then\n    action\nfi
  [[ cond ]] || action   →  if [[ ! (cond) ]]; then\n    action\nfi
  (( expr )) && action   →  if (( expr )); then\n    action\nfi
  (( expr )) || action   →  if ! (( expr )); then\n    action\nfi

Usage:
  linting/shell-autofix.py                # fix all first-party shell files
  linting/shell-autofix.py file.sh ...    # fix specific files/dirs
  linting/shell-autofix.py --dry-run      # preview without writing
"""

import json
import subprocess
import sys
from pathlib import Path

DEFAULT_PATHS = [
    "bash/",
    "zshrc/",
    "git-global-hooks/",
    "lib/tree.zsh",
    "lib/motd/",
    "lib/dotfilesctl/",
    "install.sh",
    "sync.sh",
    "uninstall.sh",
]


def run_scan(paths):
    result = subprocess.run(
        ["ast-grep", "scan", "--json"] + list(paths),
        capture_output=True,
        text=True,
    )
    if not result.stdout.strip():
        return []
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        print("error: could not parse ast-grep JSON output", file=sys.stderr)
        return []


def parse_shorthand(text):
    """Parse a shorthand test expression into components.

    Returns (kind, content, op, action) or None.
      kind    — 'bracket' for [[ ]] or 'arith' for (( ))
      content — text inside the brackets
      op      — '&&' or '||'
      action  — the command after the operator
    """
    s = text.strip()

    if s.startswith("[["):
        open_tok, close_tok, kind = "[[", "]]", "bracket"
    elif s.startswith("(("):
        open_tok, close_tok, kind = "((", "))", "arith"
    else:
        return None

    depth = 0
    i = 0
    while i < len(s):
        if s[i:i+2] == open_tok:
            depth += 1
            i += 2
        elif s[i:i+2] == close_tok:
            depth -= 1
            i += 2
            if depth == 0:
                rest = s[i:].strip()
                content = s[2:i-2].strip()
                if rest.startswith("&&"):
                    return kind, content, "&&", rest[2:].strip()
                if rest.startswith("||"):
                    return kind, content, "||", rest[2:].strip()
                return None
        else:
            i += 1

    return None


def build_replacement(indent, kind, content, op, action):
    """Build the if/then/fi replacement block."""
    body_indent = indent + "    "

    if kind == "bracket":
        if op == "&&":
            cond = f"[[ {content} ]]"
        else:
            # Wrap in parens so complex conditions (with && / ||) negate correctly
            cond = f"[[ ! ({content}) ]]"
        return f"{indent}if {cond}; then\n{body_indent}{action}\n{indent}fi"
    else:
        if op == "&&":
            return f"{indent}if (( {content} )); then\n{body_indent}{action}\n{indent}fi"
        else:
            return f"{indent}if ! (( {content} )); then\n{body_indent}{action}\n{indent}fi"


def fix_file(filepath, violations, dry_run):
    path = Path(filepath)
    lines = path.read_text().splitlines(keepends=True)

    # Work bottom-up so earlier fixes don't shift line numbers for later ones
    violations = sorted(
        violations,
        key=lambda v: v["range"]["start"]["line"],
        reverse=True,
    )

    changed = False
    for v in violations:
        start_line = v["range"]["start"]["line"]
        end_line = v["range"]["end"]["line"]
        start_col = v["range"]["start"]["column"]

        parsed = parse_shorthand(v["text"])
        if not parsed:
            print(f"  warning: could not parse — skipping: {v['text']!r}", file=sys.stderr)
            continue

        kind, content, op, action = parsed

        # Indentation = the whitespace leading up to the match on this line
        src_line = lines[start_line]
        indent = src_line[:start_col]
        if indent.strip():
            # Shouldn't happen (there's non-whitespace before the test),
            # fall back to the line's own leading whitespace
            indent = " " * (len(src_line) - len(src_line.lstrip()))

        replacement = build_replacement(indent, kind, content, op, action)

        if dry_run:
            print(f"\n  {filepath}:{start_line + 1}")
            print(f"  BEFORE: {v['text']!r}")
            for ln in replacement.splitlines():
                print(f"  AFTER:  {ln}")
            continue

        # Replace the matched span (single or multi-line)
        # The replacement is a multi-line string; store as one element — join handles it.
        new_text = indent + replacement.lstrip() + "\n"
        lines[start_line:end_line + 1] = [new_text]
        changed = True

    if changed:
        path.write_text("".join(lines))


def main():
    args = sys.argv[1:]
    dry_run = "--dry-run" in args
    paths = [a for a in args if not a.startswith("--")] or DEFAULT_PATHS

    violations = run_scan(paths)

    if not violations:
        print("No violations found.")
        return 0

    count = len(violations)
    if dry_run:
        print(f"Found {count} violation(s) (dry run — no files written):")
    else:
        print(f"Found {count} violation(s). Fixing...")

    by_file = {}
    for v in violations:
        by_file.setdefault(v["file"], []).append(v)

    for filepath, file_violations in sorted(by_file.items()):
        n = len(file_violations)
        print(f"  {filepath}: {n} fix{'es' if n != 1 else ''}")
        fix_file(filepath, file_violations, dry_run)

    if not dry_run:
        print("Done. Run 'make lint' to verify.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
