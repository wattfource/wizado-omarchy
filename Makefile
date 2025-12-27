# Wizado Makefile

VERSION ?= 1.0.0
BINARY = wizado
LDFLAGS = -s -w -X main.Version=$(VERSION)

.PHONY: all build clean install uninstall test

all: build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/wizado

install: build
	install -Dm755 $(BINARY) /usr/bin/$(BINARY)
	install -Dm755 scripts/bin/wizado-menu /usr/bin/wizado-menu
	install -Dm755 scripts/bin/wizado-menu-float /usr/bin/wizado-menu-float
	install -Dm644 scripts/config/default.conf /usr/share/$(BINARY)/default.conf
	install -Dm644 scripts/config/waybar-module.jsonc /usr/share/$(BINARY)/waybar-module.jsonc
	install -Dm644 scripts/config/waybar-style.css /usr/share/$(BINARY)/waybar-style.css

uninstall:
	rm -f /usr/bin/$(BINARY)
	rm -f /usr/bin/wizado-menu
	rm -f /usr/bin/wizado-menu-float
	rm -rf /usr/share/$(BINARY)

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

