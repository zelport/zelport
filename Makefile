GO ?= go
PYTHON ?= python3
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_REVISION := $(shell git rev-parse --short=7 HEAD 2>/dev/null)
GIT_DIRTY := $(shell git status --porcelain --untracked-files=normal 2>/dev/null)
VERSION ?= dev$(if $(GIT_REVISION),-$(GIT_REVISION)$(if $(GIT_DIRTY),-dirty))
EXTRA_LDFLAGS ?=
VERSION_LDFLAGS = $(EXTRA_LDFLAGS) -X main.version=$(VERSION) -X 'main.builtAt=$(BUILD_TIME)'
RELEASE_DIR ?= dist/$(VERSION)

.PHONY: check verify release-ready test vet build build-core check-modules check-packaging release release-assets verify-release-assets verify-published-release
check: test vet check-packaging

verify: check check-modules

release-ready: verify
	$(MAKE) release-assets
	$(MAKE) verify-release-assets

check-packaging:
	$(PYTHON) -m unittest discover -s scripts -p 'test_release*.py'
	$(PYTHON) -m unittest discover -s contrib/archlinux -p test_prepare.py

# Supply the stable release version, e.g. make release VERSION=0.10.0.
release: release-assets

release-assets:
	GO='$(GO)' $(PYTHON) scripts/release.py --version '$(VERSION)' --output '$(RELEASE_DIR)'

verify-release-assets:
	$(PYTHON) scripts/verify_release_assets.py --bundle '$(RELEASE_DIR)' --version '$(VERSION)'

verify-published-release:
	$(PYTHON) scripts/verify_published_release.py --repository zelport/zelport --version '$(VERSION)'

test:
	GOWORK=off $(GO) test -race ./...

vet:
	GOWORK=off $(GO) vet ./...

build:
	GOWORK=off $(GO) build -ldflags "$(VERSION_LDFLAGS)" -o bin/zelport ./cmd/zelport

build-core:
	GOWORK=off $(GO) build -tags zelport_core -ldflags "$(VERSION_LDFLAGS)" -o bin/zelport-core ./cmd/zelport

check-modules:
	GO='$(GO)' sh scripts/check-module-builds.sh
