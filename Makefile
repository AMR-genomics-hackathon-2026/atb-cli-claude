BINARY     := atb
BIN_DIR    := bin
CMD_PATH   := ./cmd/atb
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X main.version=$(VERSION)"

GO         := go
GOFLAGS    ?=

.PHONY: build test lint clean fixtures

build:
	mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD_PATH)

test:
	$(GO) test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BIN_DIR)

fixtures:
	$(GO) run ./internal/testdata/gen/... 2>/dev/null || true
