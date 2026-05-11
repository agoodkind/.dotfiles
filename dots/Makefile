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

# --- Smoke tests -------------------------------------------------------
# REPO_ROOT: absolute path of the dotfiles repository (parent of dots/).
REPO_ROOT := $(shell git -C $(dir $(abspath $(lastword $(MAKEFILE_LIST)))) rev-parse --show-toplevel)

TART_IMAGE_TAHOE   := ghcr.io/cirruslabs/macos-tahoe-base:latest
TART_IMAGE_SEQUOIA := ghcr.io/cirruslabs/macos-sequoia-base:latest

# Linux smokes run inside Docker containers (safe on any machine).
smoke-linux-debian:
	go run ./cmd/fresh-linux-bootstrap-smoke --image debian:trixie --repo-root $(REPO_ROOT)

smoke-linux-ubuntu:
	go run ./cmd/fresh-linux-bootstrap-smoke --image ubuntu:24.04 --repo-root $(REPO_ROOT)

smoke-linux: smoke-linux-debian smoke-linux-ubuntu

# macOS smokes run inside Tart VMs (requires: brew install cirruslabs/cli/tart).
# These are safe for developer machines; CI runners use the binary directly.
smoke-macos-tahoe:
	go run ./cmd/fresh-macos-bootstrap-smoke --tart --image $(TART_IMAGE_TAHOE) --repo-root $(REPO_ROOT)

smoke-macos-sequoia:
	go run ./cmd/fresh-macos-bootstrap-smoke --tart --image $(TART_IMAGE_SEQUOIA) --repo-root $(REPO_ROOT)

smoke-macos: smoke-macos-tahoe smoke-macos-sequoia

# smoke runs all 4 targets.
smoke: smoke-linux smoke-macos

.PHONY: smoke smoke-linux smoke-linux-debian smoke-linux-ubuntu \
        smoke-macos smoke-macos-tahoe smoke-macos-sequoia
