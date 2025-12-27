# Wizado Makefile

VERSION ?= 1.0.0
BINARY = wizado
LDFLAGS = -s -w -X main.Version=$(VERSION)

.PHONY: all build clean install test

all: build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/wizado

install: build
	install -Dm755 $(BINARY) /usr/bin/$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

# Development helpers
dev:
	go run ./cmd/wizado

deps:
	go mod download
	go mod tidy

# Build for release
release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-amd64 ./cmd/wizado

