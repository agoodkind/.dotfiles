SHELL_DIRS  := bash/ zshrc/ git-global-hooks/ lib/tree.zsh lib/motd/ lib/dotfilesctl/
SHELL_FILES := install.sh sync.sh uninstall.sh
AUTOFIX     := python3 linting/shell-autofix.py

.PHONY: lint lint-staged fix fix-staged

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
