#!/usr/bin/env bash
# Filter stdin file list, removing files shfmt cannot parse.
# Used by Makefile fmt/fmt-check targets.
#
# Add paths (relative to repo root) to SHFMT_SKIP when a file contains
# zsh-only syntax that shfmt's zsh dialect rejects (e.g. associative array
# subscripts with `:` that look like ternary operators).

SHFMT_SKIP=(
    zshrc/core/perf.zsh
)

while IFS= read -r file; do
    skip=false
    for s in "${SHFMT_SKIP[@]}"; do
        if [[ "$file" == *"$s" ]]; then
            skip=true
            break
        fi
    done
    if [[ "$skip" == false ]]; then
        printf '%s\n' "$file"
    fi
done
