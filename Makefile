GO ?= go
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_REVISION := $(shell git rev-parse --short=7 HEAD 2>/dev/null)
GIT_DIRTY := $(shell git status --porcelain --untracked-files=normal 2>/dev/null)
VERSION ?= dev$(if $(GIT_REVISION),-$(GIT_REVISION)$(if $(GIT_DIRTY),-dirty))
EXTRA_LDFLAGS ?=
VERSION_LDFLAGS = $(EXTRA_LDFLAGS) -X main.version=$(VERSION) -X 'main.builtAt=$(BUILD_TIME)'

.PHONY: check test vet build build-core check-modules check-packaging release
check: test vet check-packaging

check-packaging:
	python3 -m unittest discover -s scripts -p test_release.py
	python3 -m unittest discover -s contrib/distribution -p test_prepare.py
	bash -n install.sh
	python3 -m unittest discover -s contrib/install -p test_install.py

# Supply an explicit release version, e.g. make release VERSION=0.1.0.
release:
	GO='$(GO)' python3 scripts/release.py --version '$(VERSION)' --output 'bin/homebrew/$(VERSION)'

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
