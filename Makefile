SHELL_DIRS  := bash/ zshrc/ git-global-hooks/ lib/tree.zsh lib/motd/ lib/dotfilesctl/
SHELL_FILES := install.sh sync.sh uninstall.sh
AUTOFIX     := python3 linting/shell-autofix.py
SHFMT_FLAGS := -i 4 -ci

.PHONY: lint lint-staged fix fix-dry fix-staged fmt fmt-check fmt-staged check

# Run all checks (lint + format diff)
check: lint fmt-check

# ── ast-grep: if/then/fi style ──────────────────────────────────────────────

lint:
	ast-grep scan $(SHELL_DIRS) $(SHELL_FILES)

lint-staged:
	@staged=$$(git diff --cached --name-only --diff-filter=ACMR); \
	files=""; \
	for f in $$staged; do \
		case $$f in \
			lib/zinit/*|lib/zsh-defer/*) ;; \
			*.sh|*.bash|*.zsh) files="$$files $$f" ;; \
		esac; \
	done; \
	if [ -z "$$files" ]; then \
		echo "No staged shell files to lint."; \
		exit 0; \
	fi; \
	echo "Linting:$$files"; \
	ast-grep scan $$files

fix:
	$(AUTOFIX)

fix-dry:
	$(AUTOFIX) --dry-run

fix-staged:
	@staged=$$(git diff --cached --name-only --diff-filter=ACMR); \
	files=""; \
	for f in $$staged; do \
		case $$f in \
			lib/zinit/*|lib/zsh-defer/*) ;; \
			*.sh|*.bash|*.zsh) files="$$files $$f" ;; \
		esac; \
	done; \
	if [ -z "$$files" ]; then \
		echo "No staged shell files to fix."; \
		exit 0; \
	fi; \
	$(AUTOFIX) $$files

# ── shfmt: formatting ────────────────────────────────────────────────────────
# shfmt -f finds all shell files under the given paths; we pipe through
# linting/shfmt-files.sh which strips files it can't parse (zsh-only syntax).

fmt:
	@shfmt -f $(SHELL_DIRS) $(SHELL_FILES) \
		| linting/shfmt-filter.sh \
		| xargs shfmt $(SHFMT_FLAGS) -w

fmt-check:
	@shfmt -f $(SHELL_DIRS) $(SHELL_FILES) \
		| linting/shfmt-filter.sh \
		| xargs shfmt $(SHFMT_FLAGS) -d

fmt-staged:
	@staged=$$(git diff --cached --name-only --diff-filter=ACMR); \
	files=""; \
	for f in $$staged; do \
		case $$f in \
			lib/zinit/*|lib/zsh-defer/*) ;; \
			*.sh|*.bash|*.zsh) files="$$files $$f" ;; \
		esac; \
	done; \
	if [ -z "$$files" ]; then \
		echo "No staged shell files to format."; \
		exit 0; \
	fi; \
	printf '%s\n' $$files \
		| linting/shfmt-filter.sh \
		| xargs shfmt $(SHFMT_FLAGS) -w
