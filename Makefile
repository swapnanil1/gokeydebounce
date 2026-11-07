GO ?= go
BINARY := bin/keydebounce
VERSION ?= dev

.PHONY: build test vet fmt fmt-check check fuzz dist

build:
	CGO_ENABLED=0 $(GO) build -trimpath -o $(BINARY) ./cmd/keydebounce

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; exit 1; }

check: fmt-check vet test build

fuzz:
	$(GO) test ./internal/debounce -fuzz=FuzzFilterPairing -fuzztime=10s

dist:
	GO="$(GO)" ./scripts/package.sh "$(VERSION)"
