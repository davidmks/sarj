BINARY := bin/sarj
MODULE := github.com/davidmks/sarj
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install test test-int lint fmt clean snapshot

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/sarj

install:
	go install $(LDFLAGS) ./cmd/sarj

test:
	go test ./...

test-int:
	go test -tags integration ./...

lint:
	golangci-lint run

fmt:
	gofumpt -w .

clean:
	rm -rf bin/

snapshot:
	goreleaser release --snapshot --clean
