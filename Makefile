GO ?= go
BINARY ?= bin/openexit
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/RamazanKara/openexit/internal/version.Version=$(VERSION) -X github.com/RamazanKara/openexit/internal/version.Commit=$(COMMIT) -X github.com/RamazanKara/openexit/internal/version.Date=$(DATE)
GOFILES := $(shell find . -name '*.go' -not -path './bin/*' -not -path './dist/*')
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_LINT ?= bin/golangci-lint

.PHONY: build test fmt fmt-check lint golangci-lint verify release-dist clean

build:
	mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/openexit

test:
	$(GO) test ./...

fmt:
	gofmt -w $(GOFILES)

fmt-check:
	@test -z "$$(gofmt -l $(GOFILES))" || (echo "gofmt required:"; gofmt -l $(GOFILES); exit 1)

lint: fmt-check golangci-lint
	$(GO) vet ./...

golangci-lint:
	GOBIN=$(CURDIR)/bin $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GOLANGCI_LINT) run

verify: lint test build

release-dist:
	rm -rf dist
	mkdir -p dist
	for target in $(PLATFORMS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out="dist/openexit_$(VERSION)_$${os}_$${arch}$${ext}"; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" ./cmd/openexit; \
	done
	cd dist && sha256sum openexit_* > SHA256SUMS

clean:
	rm -rf bin dist demo openexit-demo.zip
