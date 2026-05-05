BIN_DIR ?= bin
VERSION ?= dev
GO ?= go

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build clean test release-snapshot tag-major tag-minor tag-patch

all: build

build: $(BIN_DIR)/sidecard $(BIN_DIR)/sidecarctl

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/sidecard: $(shell find cmd/sidecard internal -type f -name '*.go') go.mod go.sum | $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/sidecard

$(BIN_DIR)/sidecarctl: $(shell find cmd/sidecarctl internal -type f -name '*.go') go.mod go.sum | $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/sidecarctl

test:
	$(GO) test ./...

release-snapshot:
	goreleaser build --snapshot --clean

tag-major:
	$(MAKE) _tag BUMP=major

tag-minor:
	$(MAKE) _tag BUMP=minor

tag-patch:
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

clean:
	rm -rf $(BIN_DIR) dist
