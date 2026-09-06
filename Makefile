VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X github.com/henrwal/postatron-cli/mcpserver.Version=$(VERSION)

.PHONY: build test lint clean

build:
	go build -ldflags="$(LDFLAGS)" -o bin/postatron ./cmd/postatron
	go build -ldflags="$(LDFLAGS)" -o bin/postatron-mcp ./cmd/postatron-mcp

test:
	go test ./...

lint:
	gofmt -l . && go vet ./...

clean:
	rm -rf bin dist
