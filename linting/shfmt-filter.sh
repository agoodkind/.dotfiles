#!/usr/bin/env bash
# Exclude downloaded build dependencies and files with known shfmt issues.
# zsh-shims.zsh intentionally uses ${(flags)} syntax and is excluded.

while IFS= read -r file; do
    case "$file" in
        dots/.make/* | */dots/.make/*) ;;
        *zsh-shims.zsh) ;;
        # shfmt -ln zsh rewrites "$#" to "$" (strips the #), breaking arg-count checks.
        # zoxide.zsh uses "$#" in __zoxide_z(); exclude it until shfmt fixes the bug.
        *zoxide.zsh) ;;
        *) printf '%s\n' "$file" ;;
    esac
done
