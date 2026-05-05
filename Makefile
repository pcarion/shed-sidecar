BIN_DIR ?= bin
VERSION ?= dev
GO ?= go

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build clean test release-snapshot

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

clean:
	rm -rf $(BIN_DIR) dist
