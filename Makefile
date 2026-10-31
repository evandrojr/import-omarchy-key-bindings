BIN := import-omarchy-key-bindings
GO  ?= go

.PHONY: build test vet fmt install clean

build:
	$(GO) build -o bin/$(BIN) .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

install:
	$(GO) install .

clean:
	rm -rf bin .gocache .gopath
