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
EXAMPLE_INPUT ?= examples/datadog-to-grafana/input/datadog-fixture.json
EXAMPLE_DIR ?= examples/datadog-to-grafana/output
EXAMPLE_BUNDLE ?= examples/datadog-to-grafana/openexit-example.zip

.PHONY: build test fmt fmt-check lint golangci-lint smoke example example-smoke verify release-dist release-check clean

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
	$(BINARY) demo "$$tmp/builtin-demo" --out "$$tmp/builtin-demo.zip"; \
	test -s "$$tmp/builtin-demo.zip"; \
	$(BINARY) init "$$tmp/datadog-demo"; \
	$(BINARY) collect fixture --project "$$tmp/datadog-demo" --input ./testdata/datadog/small.json; \
	$(BINARY) assess --project "$$tmp/datadog-demo" --target grafana-lgtm; \
	$(BINARY) map --project "$$tmp/datadog-demo"; \
	$(BINARY) generate --project "$$tmp/datadog-demo" --all; \
	$(BINARY) validate --project "$$tmp/datadog-demo"; \
	$(BINARY) export --project "$$tmp/datadog-demo" --format zip --out "$$tmp/openexit-demo.zip"; \
	$(BINARY) init "$$tmp/ghe-demo" --source github-enterprise --target forgejo; \
	$(BINARY) collect github-fixture --project "$$tmp/ghe-demo" --input ./testdata/github-enterprise/small.json; \
	$(BINARY) assess --project "$$tmp/ghe-demo" --target forgejo; \
	$(BINARY) map --project "$$tmp/ghe-demo"; \
	$(BINARY) generate --project "$$tmp/ghe-demo" --all; \
	$(BINARY) validate --project "$$tmp/ghe-demo"; \
	$(BINARY) export --project "$$tmp/ghe-demo" --format zip --out "$$tmp/ghe-demo.zip"; \
	$(BINARY) init "$$tmp/identity-demo" --source identity --target keycloak-zitadel; \
	$(BINARY) collect identity-fixture --project "$$tmp/identity-demo" --input ./testdata/identity/small.json; \
	$(BINARY) assess --project "$$tmp/identity-demo" --target keycloak-zitadel; \
	$(BINARY) map --project "$$tmp/identity-demo"; \
	$(BINARY) generate --project "$$tmp/identity-demo" --all; \
	$(BINARY) validate --project "$$tmp/identity-demo"; \
	$(BINARY) export --project "$$tmp/identity-demo" --format zip --out "$$tmp/identity-demo.zip"; \
	$(BINARY) init "$$tmp/edge-demo" --source edge --target varnish-haproxy-coraza; \
	$(BINARY) collect edge-fixture --project "$$tmp/edge-demo" --input ./testdata/edge/small.json; \
	$(BINARY) assess --project "$$tmp/edge-demo" --target varnish-haproxy-coraza; \
	$(BINARY) map --project "$$tmp/edge-demo"; \
	$(BINARY) generate --project "$$tmp/edge-demo" --all; \
	$(BINARY) validate --project "$$tmp/edge-demo"; \
	$(BINARY) export --project "$$tmp/edge-demo" --format zip --out "$$tmp/edge-demo.zip"; \
	$(BINARY) init "$$tmp/ai-demo" --source ai-provider --target vllm-litellm; \
	$(BINARY) collect ai-fixture --project "$$tmp/ai-demo" --input ./testdata/ai-provider/small.json; \
	$(BINARY) assess --project "$$tmp/ai-demo" --target vllm-litellm; \
	$(BINARY) map --project "$$tmp/ai-demo"; \
	$(BINARY) generate --project "$$tmp/ai-demo" --all; \
	$(BINARY) validate --project "$$tmp/ai-demo"; \
	$(BINARY) export --project "$$tmp/ai-demo" --format zip --out "$$tmp/ai-demo.zip"

example: build
	rm -rf $(EXAMPLE_DIR)/openexit.yaml \
		$(EXAMPLE_DIR)/inventory \
		$(EXAMPLE_DIR)/assessment \
		$(EXAMPLE_DIR)/mapping \
		$(EXAMPLE_DIR)/generated-config \
		$(EXAMPLE_DIR)/evidence \
		$(EXAMPLE_DIR)/validation \
		$(EXAMPLE_BUNDLE)
	mkdir -p $(EXAMPLE_DIR)
	$(BINARY) init $(EXAMPLE_DIR) --source datadog --target grafana-lgtm
	$(BINARY) collect fixture --project $(EXAMPLE_DIR) --input $(EXAMPLE_INPUT)
	$(BINARY) assess --project $(EXAMPLE_DIR) --target grafana-lgtm
	$(BINARY) map --project $(EXAMPLE_DIR)
	$(BINARY) generate --project $(EXAMPLE_DIR) --all
	$(BINARY) validate --project $(EXAMPLE_DIR)
	$(BINARY) export --project $(EXAMPLE_DIR) --format zip --out $(EXAMPLE_BUNDLE)

example-smoke: build
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	$(BINARY) init "$$tmp/example" --source datadog --target grafana-lgtm; \
	$(BINARY) collect fixture --project "$$tmp/example" --input ./$(EXAMPLE_INPUT); \
	$(BINARY) assess --project "$$tmp/example" --target grafana-lgtm; \
	$(BINARY) map --project "$$tmp/example"; \
	$(BINARY) generate --project "$$tmp/example" --all; \
	$(BINARY) validate --project "$$tmp/example"; \
	$(BINARY) export --project "$$tmp/example" --format zip --out "$$tmp/openexit-example.zip"; \
	test -s "$$tmp/openexit-example.zip"

verify: lint test build smoke example-smoke

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

release-check: verify release-dist
	@test -s dist/SHA256SUMS
	@expected=$$(printf '%s\n' $(PLATFORMS) | wc -w | tr -d ' '); \
	actual=$$(wc -l < dist/SHA256SUMS | tr -d ' '); \
	if [ "$$actual" != "$$expected" ]; then \
		echo "expected $$expected release checksums, got $$actual"; \
		exit 1; \
	fi
	@$(BINARY) version | grep -q 'name: openexit'
	@$(BINARY) version | grep -q 'version: $(VERSION)'
	@echo "release check passed for $(VERSION)"

clean:
	rm -rf bin dist demo openexit-demo.zip
