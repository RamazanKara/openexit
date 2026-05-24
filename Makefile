GO ?= go
BINARY ?= bin/openexit

.PHONY: build test lint clean

build:
	$(GO) build -o $(BINARY) ./cmd/openexit

test:
	$(GO) test ./...

lint:
	$(GO)fmt -w .
	$(GO) vet ./...

clean:
	rm -rf bin demo openexit-demo.zip
