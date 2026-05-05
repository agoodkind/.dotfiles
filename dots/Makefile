# Lint is centralized in go-makefile. Do NOT define project-local lint,
# deadcode, audit, fmt, vet, or staticcheck targets here. They duplicate
# the central pipeline and let agents bypass strict rules. Run `make help`
# for the canonical entry points (build/check/lint/fmt) and per-linter
# sub-targets (lint-golangci, lint-format, lint-gocyclo, lint-deadcode,
# staticcheck-extra). Refresh baselines via the matching *-baseline target.
#
# dots Makefile.
# Build/lint pipeline lives in go-makefile and is fetched at runtime.
# dots ships via bootstrap-go.sh (which `go run`s from source); release-mk
# is intentionally omitted.

# Identity. No VPKG: dots has no version package; build metadata is not
# stamped because dots is `go run`-from-source, not a distributed binary.
BINARY := dots
CMD    := ./cmd/$(BINARY)

# Pipeline modules
GO_MK_MODULES := go-build.mk

include bootstrap.mk

.DEFAULT_GOAL := check
