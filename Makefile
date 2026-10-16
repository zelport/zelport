GO ?= go
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_REVISION := $(shell git rev-parse --short=7 HEAD 2>/dev/null)
GIT_DIRTY := $(shell git status --porcelain --untracked-files=normal 2>/dev/null)
VERSION ?= dev$(if $(GIT_REVISION),-$(GIT_REVISION)$(if $(GIT_DIRTY),-dirty))
EXTRA_LDFLAGS ?=
VERSION_LDFLAGS = $(EXTRA_LDFLAGS) -X main.version=$(VERSION) -X 'main.builtAt=$(BUILD_TIME)'
FEATURE_TAGS := docker_sandboxes,containers,local_ports,remote_http,ports_over_ssh,ports_over_relay,built_in_relay

.PHONY: check test vet build build-core build-full check-modules check-packaging release
check: test vet check-packaging

check-packaging:
	python3 -m unittest discover -s scripts -p test_release.py
	python3 -m unittest discover -s contrib/distribution -p test_prepare.py

# Supply an explicit release version, e.g. make release VERSION=0.1.0.
release:
	GO='$(GO)' python3 scripts/release.py --version '$(VERSION)' --tags '$(FEATURE_TAGS)' --output 'bin/homebrew/$(VERSION)'

test:
	GOWORK=off $(GO) test -race ./...
	GOWORK=off $(GO) test -race -tags '$(FEATURE_TAGS)' ./...

vet:
	GOWORK=off $(GO) vet ./...
	GOWORK=off $(GO) vet -tags '$(FEATURE_TAGS)' ./...

build: build-full

build-core:
	GOWORK=off $(GO) build -ldflags "$(VERSION_LDFLAGS)" -o bin/zelport-core ./cmd/zelport

build-full:
	GOWORK=off $(GO) build -tags '$(FEATURE_TAGS)' -ldflags "$(VERSION_LDFLAGS)" -o bin/zelport ./cmd/zelport

check-modules:
	GO='$(GO)' FULL_TAGS='$(FEATURE_TAGS)' sh scripts/check-module-builds.sh
