# dots Makefile.
# Build/lint pipeline lives in go-makefile and is fetched at runtime.
# dots ships via bootstrap-go.sh (which `go run`s from source); release-mk
# is intentionally omitted.

GO_MK_URL     := https://raw.githubusercontent.com/agoodkind/go-makefile/main/go.mk
GO_MK_API_URL := https://api.github.com/repos/agoodkind/go-makefile/contents/go.mk?ref=main
GO_MK         := .make/go.mk
GO_MK_CACHE   := $(or $(XDG_CACHE_HOME),$(HOME)/.cache)/go-makefile/go.mk
# Dev override: GO_MK_DEV_DIR=$HOME/Sites/go-makefile to iterate locally.
GO_MK_DEV_DIR ?=

# Identity. No VPKG: dots has no version package; build metadata is not
# stamped because dots is `go run`-from-source, not a distributed binary.
BINARY := dots
CMD    := ./cmd/$(BINARY)

# Pipeline modules
GO_MK_MODULES := go-build.mk

GO_MK_BOOTSTRAP := $(shell \
	mkdir -p "$(dir $(GO_MK))" "$(dir $(GO_MK_CACHE))"; \
	if [ -n "$(GO_MK_DEV_DIR)" ] && [ -f "$(GO_MK_DEV_DIR)/go.mk" ]; then \
		cp "$(GO_MK_DEV_DIR)/go.mk" "$(GO_MK)"; \
		printf '%s\n' "go.mk: using dev override $(GO_MK_DEV_DIR)/go.mk" >&2; \
	else \
		tmp="$(GO_MK).tmp"; \
		if curl -fsSL -H "Accept: application/vnd.github.raw" --connect-timeout 5 --max-time 10 "$(GO_MK_API_URL)" -o "$$tmp" || curl -fsSL --connect-timeout 5 --max-time 10 "$(GO_MK_URL)?v=$$(date +%s)" -o "$$tmp" || curl -fsSL --connect-timeout 5 --max-time 10 "$(GO_MK_URL)" -o "$$tmp"; then \
			mv "$$tmp" "$(GO_MK)"; \
			cp "$(GO_MK)" "$(GO_MK_CACHE)"; \
		elif [ -f "$(GO_MK_CACHE)" ]; then \
			rm -f "$$tmp"; \
			cp "$(GO_MK_CACHE)" "$(GO_MK)"; \
		elif [ ! -f "$(GO_MK)" ]; then \
			rm -f "$$tmp"; \
			printf '%s\n' "error: go.mk fetch failed and no cache available" >&2; \
		fi; \
	fi)

$(GO_MK):
	@mkdir -p $(dir $@)
	@if [ -n "$(GO_MK_DEV_DIR)" ] && [ -f "$(GO_MK_DEV_DIR)/go.mk" ]; then \
		cp "$(GO_MK_DEV_DIR)/go.mk" "$@"; \
		echo "go.mk: using dev override $(GO_MK_DEV_DIR)/go.mk" >&2; \
	elif curl -fsSL -H "Accept: application/vnd.github.raw" --connect-timeout 5 --max-time 10 "$(GO_MK_API_URL)" -o "$@" || curl -fsSL --connect-timeout 5 --max-time 10 "$(GO_MK_URL)?v=$$(date +%s)" -o "$@" || curl -fsSL --connect-timeout 5 --max-time 10 "$(GO_MK_URL)" -o "$@"; then \
		mkdir -p "$(dir $(GO_MK_CACHE))" && cp "$@" "$(GO_MK_CACHE)"; \
	elif [ -f "$(GO_MK_CACHE)" ]; then \
		echo "warning: go.mk fetch failed, using cached version" >&2; \
		cp "$(GO_MK_CACHE)" "$@"; \
	else \
		echo "error: go.mk fetch failed and no cache available" >&2; \
		exit 1; \
	fi

-include $(GO_MK)

.DEFAULT_GOAL := check
