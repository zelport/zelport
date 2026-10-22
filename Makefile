GO ?= go
PYTHON ?= python3
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_REVISION := $(shell git rev-parse --short=7 HEAD 2>/dev/null)
GIT_DIRTY := $(shell git status --porcelain --untracked-files=normal 2>/dev/null)
VERSION ?= dev$(if $(GIT_REVISION),-$(GIT_REVISION)$(if $(GIT_DIRTY),-dirty))
BUILD_VERSION := $(patsubst v%,%,$(VERSION))
EXTRA_LDFLAGS ?=
VERSION_LDFLAGS = $(EXTRA_LDFLAGS) -X main.version=$(BUILD_VERSION) -X 'main.builtAt=$(BUILD_TIME)'
RELEASE_DIR ?= dist/$(BUILD_VERSION)

.DEFAULT_GOAL := check
.PHONY: help check verify release-ready test test-browser vet build build-core check-modules check-packaging release release-assets verify-release-assets verify-published-release

help:
	@printf '%s\n' 'make check' 'make verify' 'make build' 'make release-ready VERSION=vX.Y.Z' 'make verify-published-release VERSION=vX.Y.Z'

check: test vet check-packaging

verify: check check-modules

release-ready: verify
	$(MAKE) release-assets
	$(MAKE) verify-release-assets

check-packaging:
	$(PYTHON) -m unittest discover -s scripts -p 'test_release*.py'
	$(PYTHON) -m unittest discover -s contrib/archlinux -p test_prepare.py

# Supply the canonical stable release, e.g. make release VERSION=v0.10.0.
release: release-assets

release-assets:
	GO='$(GO)' $(PYTHON) scripts/release.py --version '$(BUILD_VERSION)' --output '$(RELEASE_DIR)'

verify-release-assets:
	$(PYTHON) scripts/verify_release_assets.py --bundle '$(RELEASE_DIR)' --version '$(BUILD_VERSION)'

verify-published-release:
	$(PYTHON) scripts/verify_published_release.py --repository zelport/zelport --version '$(BUILD_VERSION)'

test: test-browser
	GOWORK=off $(GO) test -race ./...

test-browser:
	node --test scripts/*.test.mjs

vet:
	GOWORK=off $(GO) vet ./...

build:
	GOWORK=off $(GO) build -ldflags "$(VERSION_LDFLAGS)" -o bin/zelport ./cmd/zelport

build-core:
	GOWORK=off $(GO) build -tags zelport_core -ldflags "$(VERSION_LDFLAGS)" -o bin/zelport-core ./cmd/zelport

check-modules:
	GO='$(GO)' sh scripts/check-module-builds.sh
