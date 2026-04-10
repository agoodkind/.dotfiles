SHELL_DIRS := bash/ zshrc/ git-global-hooks/ lib/tree.zsh lib/motd/ lib/dotfilesctl/
SHELL_FILES := install.sh sync.sh uninstall.sh

.PHONY: lint lint-staged

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
