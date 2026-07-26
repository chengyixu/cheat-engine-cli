BINARY := cecli
PACKAGE := ./cmd/cecli
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/chengyixu/cheat-engine-cli/internal/cli.Version=$(VERSION) \
	-X github.com/chengyixu/cheat-engine-cli/internal/cli.Commit=$(COMMIT) \
	-X github.com/chengyixu/cheat-engine-cli/internal/cli.BuildDate=$(BUILD_DATE)

.PHONY: all build install test race vet check smoke clean snapshot

all: check build

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PACKAGE)

install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(PACKAGE)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

check: test vet

smoke: build
	bin/$(BINARY) --help | jq -e '.ok == true and .command == "help"' >/dev/null
	bin/$(BINARY) --version | jq -e '.data.name == "cecli"' >/dev/null
	bin/$(BINARY) memory write --pid 42 --address 0x1000 --type i32 --value 100 --dry-run | jq -e '.data.bytes_hex == "64000000"' >/dev/null

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
