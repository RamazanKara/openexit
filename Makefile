GO ?= go
BINARY ?= bin/openexit
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/RamazanKara/openexit/internal/version.Version=$(VERSION) -X github.com/RamazanKara/openexit/internal/version.Commit=$(COMMIT) -X github.com/RamazanKara/openexit/internal/version.Date=$(DATE)
GOFILES := $(shell find . -name '*.go' -not -path './bin/*' -not -path './dist/*')
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
RELEASE_MANIFEST ?= RELEASE_MANIFEST.json
RELEASE_SBOM ?= SBOM.cdx.json
RELEASE_ASSETS ?= install.sh openexit.bash _openexit openexit.fish openexit.ps1 $(RELEASE_SBOM)
GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_LINT ?= bin/golangci-lint
EXAMPLE_INPUT ?= examples/datadog-to-grafana/input/datadog-fixture.json
EXAMPLE_STATE ?= examples/datadog-to-grafana/.openexit
EXAMPLE_DIR ?= examples/datadog-to-grafana/migration
README_DEMO_TAPE ?= docs/assets/openexit-demo.tape

.PHONY: build test fmt fmt-check lint golangci-lint smoke experimental-smoke example example-smoke readme-demo verify release-dist install-smoke release-check clean

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
	$(BINARY) doctor; \
	$(BINARY) datadog scan --fixture ./testdata/datadog/small.json --workdir "$$tmp/.openexit"; \
	$(BINARY) datadog plan --target grafana-lgtm --workdir "$$tmp/.openexit"; \
	$(BINARY) datadog export --out "$$tmp/migration" --workdir "$$tmp/.openexit"; \
	test -s "$$tmp/migration/index.html"; \
	test -s "$$tmp/migration/manifest.json"; \
	test -s "$$tmp/migration/SHA256SUMS"; \
	! grep -R 'vector(0)' "$$tmp/migration/generated"

experimental-smoke:
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	$(BINARY) experimental demo "$$tmp/builtin-demo" --out "$$tmp/builtin-demo.zip"; \
	test -s "$$tmp/builtin-demo.zip"; \
	$(BINARY) verify-bundle "$$tmp/builtin-demo.zip"; \
	$(BINARY) experimental init "$$tmp/datadog-demo"; \
	$(BINARY) experimental collect fixture --project "$$tmp/datadog-demo" --input ./testdata/datadog/small.json; \
	$(BINARY) experimental assess --project "$$tmp/datadog-demo" --target grafana-lgtm; \
	$(BINARY) experimental map --project "$$tmp/datadog-demo"; \
	$(BINARY) experimental generate --project "$$tmp/datadog-demo" --all; \
	$(BINARY) experimental validate --project "$$tmp/datadog-demo"; \
	$(BINARY) experimental export --project "$$tmp/datadog-demo" --format zip --out "$$tmp/openexit-demo.zip"; \
	$(BINARY) verify-bundle "$$tmp/openexit-demo.zip"; \
	$(BINARY) experimental init "$$tmp/ghe-demo" --source github-enterprise --target forgejo; \
	$(BINARY) experimental collect github-fixture --project "$$tmp/ghe-demo" --input ./testdata/github-enterprise/small.json; \
	$(BINARY) experimental assess --project "$$tmp/ghe-demo" --target forgejo; \
	$(BINARY) experimental map --project "$$tmp/ghe-demo"; \
	$(BINARY) experimental generate --project "$$tmp/ghe-demo" --all; \
	$(BINARY) experimental validate --project "$$tmp/ghe-demo"; \
	$(BINARY) experimental export --project "$$tmp/ghe-demo" --format zip --out "$$tmp/ghe-demo.zip"; \
	$(BINARY) verify-bundle "$$tmp/ghe-demo.zip"; \
	$(BINARY) experimental init "$$tmp/identity-demo" --source identity --target keycloak-zitadel; \
	$(BINARY) experimental collect identity-fixture --project "$$tmp/identity-demo" --input ./testdata/identity/small.json; \
	$(BINARY) experimental assess --project "$$tmp/identity-demo" --target keycloak-zitadel; \
	$(BINARY) experimental map --project "$$tmp/identity-demo"; \
	$(BINARY) experimental generate --project "$$tmp/identity-demo" --all; \
	$(BINARY) experimental validate --project "$$tmp/identity-demo"; \
	$(BINARY) experimental export --project "$$tmp/identity-demo" --format zip --out "$$tmp/identity-demo.zip"; \
	$(BINARY) verify-bundle "$$tmp/identity-demo.zip"; \
	$(BINARY) experimental init "$$tmp/edge-demo" --source edge --target varnish-haproxy-coraza; \
	$(BINARY) experimental collect edge-fixture --project "$$tmp/edge-demo" --input ./testdata/edge/small.json; \
	$(BINARY) experimental assess --project "$$tmp/edge-demo" --target varnish-haproxy-coraza; \
	$(BINARY) experimental map --project "$$tmp/edge-demo"; \
	$(BINARY) experimental generate --project "$$tmp/edge-demo" --all; \
	$(BINARY) experimental validate --project "$$tmp/edge-demo"; \
	$(BINARY) experimental export --project "$$tmp/edge-demo" --format zip --out "$$tmp/edge-demo.zip"; \
	$(BINARY) verify-bundle "$$tmp/edge-demo.zip"; \
	$(BINARY) experimental init "$$tmp/ai-demo" --source ai-provider --target vllm-litellm; \
	$(BINARY) experimental collect ai-fixture --project "$$tmp/ai-demo" --input ./testdata/ai-provider/small.json; \
	$(BINARY) experimental assess --project "$$tmp/ai-demo" --target vllm-litellm; \
	$(BINARY) experimental map --project "$$tmp/ai-demo"; \
	$(BINARY) experimental generate --project "$$tmp/ai-demo" --all; \
	$(BINARY) experimental validate --project "$$tmp/ai-demo"; \
	$(BINARY) experimental export --project "$$tmp/ai-demo" --format zip --out "$$tmp/ai-demo.zip"; \
	$(BINARY) verify-bundle "$$tmp/ai-demo.zip"

example: build
	$(BINARY) datadog scan --fixture $(EXAMPLE_INPUT) --workdir $(EXAMPLE_STATE)
	$(BINARY) datadog plan --target grafana-lgtm --workdir $(EXAMPLE_STATE)
	$(BINARY) datadog export --force --out $(EXAMPLE_DIR) --workdir $(EXAMPLE_STATE)

example-smoke: build
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	$(BINARY) datadog scan --fixture ./$(EXAMPLE_INPUT) --workdir "$$tmp/.openexit"; \
	$(BINARY) datadog plan --target grafana-lgtm --workdir "$$tmp/.openexit"; \
	$(BINARY) datadog export --out "$$tmp/migration" --workdir "$$tmp/.openexit"; \
	test -s "$$tmp/migration/index.html"; \
	test -s "$$tmp/migration/manifest.json"

readme-demo: build
	@command -v vhs >/dev/null 2>&1 || (echo "vhs is required: https://github.com/charmbracelet/vhs"; exit 1)
	vhs $(README_DEMO_TAPE)

verify: lint test build smoke experimental-smoke example-smoke

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
	cp scripts/install.sh dist/install.sh
	chmod +x dist/install.sh
	$(GO) run -trimpath -ldflags "$(LDFLAGS)" ./cmd/openexit completion bash > dist/openexit.bash
	$(GO) run -trimpath -ldflags "$(LDFLAGS)" ./cmd/openexit completion zsh > dist/_openexit
	$(GO) run -trimpath -ldflags "$(LDFLAGS)" ./cmd/openexit completion fish > dist/openexit.fish
	$(GO) run -trimpath -ldflags "$(LDFLAGS)" ./cmd/openexit completion powershell > dist/openexit.ps1
	$(GO) run -trimpath -ldflags "$(LDFLAGS)" ./cmd/openexit sbom --out dist/$(RELEASE_SBOM)
	$(GO) run -trimpath -ldflags "$(LDFLAGS)" ./cmd/openexit release-manifest --dist dist --out dist/$(RELEASE_MANIFEST) $(foreach target,$(PLATFORMS),--platform $(target)) $(foreach asset,$(RELEASE_ASSETS),--asset $(asset))
	cd dist && sha256sum openexit_* $(RELEASE_ASSETS) > SHA256SUMS

install-smoke: release-dist
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	OPENEXIT_VERSION=$(VERSION) OPENEXIT_BASE_URL=$(CURDIR)/dist BIN_DIR="$$tmp/bin" sh scripts/install.sh; \
	"$$tmp/bin/openexit" version | grep -q 'version: $(VERSION)'

release-check: verify release-dist install-smoke
	@test -s dist/SHA256SUMS
	@test -s dist/$(RELEASE_MANIFEST)
	@test -x dist/install.sh
	@test -s dist/openexit.bash
	@test -s dist/_openexit
	@test -s dist/openexit.fish
	@test -s dist/openexit.ps1
	@test -s dist/$(RELEASE_SBOM)
	@expected=$$(printf '%s\n' $(PLATFORMS) $(RELEASE_ASSETS) | wc -w | tr -d ' '); \
	actual=$$(wc -l < dist/SHA256SUMS | tr -d ' '); \
	if [ "$$actual" != "$$expected" ]; then \
		echo "expected $$expected release checksums, got $$actual"; \
		exit 1; \
	fi
	@$(BINARY) verify-release dist/$(RELEASE_MANIFEST) --dist dist --require-checksums
	@$(BINARY) version | grep -q 'name: openexit'
	@$(BINARY) version | grep -q 'version: $(VERSION)'
	@echo "release check passed for $(VERSION)"

clean:
	rm -rf bin dist demo openexit-demo.zip
