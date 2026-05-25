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

.PHONY: build test fmt fmt-check lint golangci-lint smoke verify release-dist clean

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

smoke:
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	$(BINARY) init "$$tmp/datadog-demo"; \
	$(BINARY) collect fixture --project "$$tmp/datadog-demo" --input ./testdata/datadog/small.json; \
	$(BINARY) assess --project "$$tmp/datadog-demo" --target grafana-lgtm; \
	$(BINARY) generate --project "$$tmp/datadog-demo" --all; \
	$(BINARY) validate --project "$$tmp/datadog-demo"; \
	$(BINARY) export --project "$$tmp/datadog-demo" --format zip --out "$$tmp/openexit-demo.zip"; \
	$(BINARY) init "$$tmp/ghe-demo" --source github-enterprise --target forgejo; \
	$(BINARY) collect github-fixture --project "$$tmp/ghe-demo" --input ./testdata/github-enterprise/small.json; \
	$(BINARY) assess --project "$$tmp/ghe-demo" --target forgejo; \
	$(BINARY) generate --project "$$tmp/ghe-demo" --all; \
	$(BINARY) validate --project "$$tmp/ghe-demo"; \
	$(BINARY) init "$$tmp/identity-demo" --source identity --target keycloak-zitadel; \
	$(BINARY) collect identity-fixture --project "$$tmp/identity-demo" --input ./testdata/identity/small.json; \
	$(BINARY) assess --project "$$tmp/identity-demo" --target keycloak-zitadel; \
	$(BINARY) generate --project "$$tmp/identity-demo" --all; \
	$(BINARY) validate --project "$$tmp/identity-demo"; \
	$(BINARY) init "$$tmp/edge-demo" --source edge --target varnish-haproxy-coraza; \
	$(BINARY) collect edge-fixture --project "$$tmp/edge-demo" --input ./testdata/edge/small.json; \
	$(BINARY) assess --project "$$tmp/edge-demo" --target varnish-haproxy-coraza; \
	$(BINARY) generate --project "$$tmp/edge-demo" --all; \
	$(BINARY) validate --project "$$tmp/edge-demo"; \
	$(BINARY) init "$$tmp/ai-demo" --source ai-provider --target vllm-litellm; \
	$(BINARY) collect ai-fixture --project "$$tmp/ai-demo" --input ./testdata/ai-provider/small.json; \
	$(BINARY) assess --project "$$tmp/ai-demo" --target vllm-litellm; \
	$(BINARY) generate --project "$$tmp/ai-demo" --all; \
	$(BINARY) validate --project "$$tmp/ai-demo"

verify: lint test build smoke

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
