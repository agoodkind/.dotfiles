#!/usr/bin/env bash
# Filter stdin file list removing files shfmt cannot parse with -ln zsh.
# zsh-shims.zsh intentionally uses ${(flags)} syntax and is excluded.

while IFS= read -r file; do
    case "$file" in
        *zsh-shims.zsh) ;;
        # shfmt -ln zsh rewrites "$#" to "$" (strips the #), breaking arg-count checks.
        # zoxide.zsh uses "$#" in __zoxide_z(); exclude it until shfmt fixes the bug.
        *zoxide.zsh) ;;
        *) printf '%s\n' "$file" ;;
    esac
done
