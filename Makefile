# ---------------------------------------------------------------
# SmartThings MCP — Makefile
# ---------------------------------------------------------------
# Usage:
#   make              Build the binary for the current platform
#   make build        Same as above
#   make all          Build linux + windows binaries
#   make test         Run tests
#   make docker       Build Docker image (load locally)
#   make docker-push  Build + push Docker image to registry
#   make run          Build and run locally (stdio)
#   make run-http     Build and run locally (stream/HTTP)
#   make clean        Remove build artifacts
#   make version      Print current version
#
#   make release-patch   Bump patch, commit, tag (local)
#   make release-minor   Bump minor, commit, tag (local)
#   make release-major   Bump major, commit, tag (local)
#   make publish-patch   Bump patch + push (triggers CI release)
#   make publish-minor   Bump minor + push (triggers CI release)
#   make publish-major   Bump major + push (triggers CI release)
# ---------------------------------------------------------------

# Version from source
VERSION   := $(shell grep -oP 'Version\s*=\s*"\K[^"]+' internal/version/version.go)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build settings
MODULE    := github.com/langowarny/smartthings-mcp
LDFLAGS_PKG := $(MODULE)/internal/version
LDFLAGS   := -s -w \
             -X $(LDFLAGS_PKG).Version=$(VERSION) \
             -X $(LDFLAGS_PKG).Commit=$(COMMIT) \
             -X $(LDFLAGS_PKG).Date=$(DATE)
BINARY    := smartthings-mcp
DIST      := dist

# Docker settings
REGISTRY  ?= ghcr.io
REGISTRY_ORG ?= ravensorb
IMAGE     := $(REGISTRY)/$(REGISTRY_ORG)/$(BINARY)

# ---------------------------------------------------------------
# Default
# ---------------------------------------------------------------
.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags='$(LDFLAGS)' -o $(DIST)/$(BINARY) ./cmd/server

# ---------------------------------------------------------------
# Cross-compile
# ---------------------------------------------------------------
.PHONY: all
all: build-linux build-windows

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='$(LDFLAGS)' \
		-o $(DIST)/$(BINARY)-linux-amd64 ./cmd/server

.PHONY: build-windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(LDFLAGS)' \
		-o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/server

# ---------------------------------------------------------------
# Test
# ---------------------------------------------------------------
.PHONY: test
test:
	go test -v -race ./...

.PHONY: vet
vet:
	go vet ./...

# ---------------------------------------------------------------
# Run
# ---------------------------------------------------------------
.PHONY: run
run: build
	$(DIST)/$(BINARY) -transport stdio

.PHONY: run-http
run-http: build
	$(DIST)/$(BINARY) -transport stream -port 8081

# ---------------------------------------------------------------
# Docker
# ---------------------------------------------------------------
.PHONY: docker
docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(DATE) \
		-t $(IMAGE):dev \
		-t $(IMAGE):dev-$(COMMIT) \
		.

.PHONY: docker-push
docker-push:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(DATE) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		--push \
		.

# ---------------------------------------------------------------
# Release (local only — commit + tag, no push)
# ---------------------------------------------------------------
.PHONY: release-patch release-minor release-major
release-patch:
	@./scripts/release.sh patch
release-minor:
	@./scripts/release.sh minor
release-major:
	@./scripts/release.sh major

# ---------------------------------------------------------------
# Publish (commit + tag + push — triggers CI release)
# ---------------------------------------------------------------
.PHONY: publish-patch publish-minor publish-major
publish-patch:
	@./scripts/release.sh patch --push
publish-minor:
	@./scripts/release.sh minor --push
publish-major:
	@./scripts/release.sh major --push

# ---------------------------------------------------------------
# Utilities
# ---------------------------------------------------------------
.PHONY: version
version:
	@echo $(VERSION)

.PHONY: clean
clean:
	rm -rf $(DIST)

.PHONY: help
help:
	@sed -n '/^# Usage:/,/^# ---/{ /^# ---/d; s/^# \{0,2\}//; p }' Makefile
