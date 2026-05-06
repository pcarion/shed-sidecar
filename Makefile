BIN_DIR ?= bin
VERSION ?= dev
GO ?= go

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all help build clean test release-snapshot tag-major tag-minor tag-patch

all: build

help: ## Show this help.
	@awk 'BEGIN { FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n" } /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-18s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: $(BIN_DIR)/shed-sidecard $(BIN_DIR)/shed-sidecar ## Build shed-sidecard and shed-sidecar into bin/.

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/shed-sidecard: $(shell find cmd/sidecard internal -type f -name '*.go') go.mod go.sum | $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/sidecard

$(BIN_DIR)/shed-sidecar: $(shell find cmd/sidecarctl internal -type f -name '*.go') go.mod go.sum | $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/sidecarctl

test: ## Run all Go tests.
	$(GO) test ./...

release-snapshot: ## Build local snapshot release artifacts with GoReleaser.
	goreleaser build --snapshot --clean

tag-major: ## Create and push the next major version tag.
	$(MAKE) _tag BUMP=major

tag-minor: ## Create and push the next minor version tag.
	$(MAKE) _tag BUMP=minor

tag-patch: ## Create and push the next patch version tag.
	$(MAKE) _tag BUMP=patch

.PHONY: _tag
_tag:
	@if [ -z "$(BUMP)" ]; then echo "BUMP is required" >&2; exit 2; fi; \
	git fetch --tags --quiet; \
	latest=$$(git tag --list 'v*' | awk '/^v[0-9]+\.[0-9]+\.[0-9]+$$/ { sub(/^v/, ""); print }' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n 1); \
	if [ -z "$$latest" ]; then latest=0.0.0; fi; \
	IFS=. ; set -- $$latest; major=$$1; minor=$$2; patch=$$3; \
	case "$(BUMP)" in \
		major) major=$$((major + 1)); minor=0; patch=0 ;; \
		minor) minor=$$((minor + 1)); patch=0 ;; \
		patch) patch=$$((patch + 1)) ;; \
		*) echo "unknown BUMP value: $(BUMP)" >&2; exit 2 ;; \
	esac; \
	tag="v$$major.$$minor.$$patch"; \
	if git rev-parse -q --verify "refs/tags/$$tag" >/dev/null; then echo "tag $$tag already exists" >&2; exit 1; fi; \
	echo "Creating and pushing $$tag"; \
	git tag -a "$$tag" -m "Release $$tag"; \
	git push origin "$$tag"

clean: ## Remove generated build and release artifacts.
	rm -rf $(BIN_DIR) dist
