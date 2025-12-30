# Wizado Makefile

VERSION ?= 1.0.1
BINARY = wizado
LDFLAGS = -s -w -X main.Version=$(VERSION)

.PHONY: all build clean install uninstall reinstall test dev deps release

all: build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/wizado

install: build
	@echo "Cleaning up previous installation..."
	@rm -f /usr/bin/$(BINARY) 2>/dev/null || true
	@rm -f /usr/bin/wizado-menu-float 2>/dev/null || true
	@rm -rf /usr/share/$(BINARY) 2>/dev/null || true
	@echo "Installing $(BINARY) v$(VERSION)..."
	install -Dm755 $(BINARY) /usr/bin/$(BINARY)
	install -Dm755 scripts/bin/wizado-menu-float /usr/bin/wizado-menu-float
	install -Dm644 scripts/config/default.conf /usr/share/$(BINARY)/default.conf
	install -Dm644 scripts/config/waybar-module.jsonc /usr/share/$(BINARY)/waybar-module.jsonc
	install -Dm644 scripts/config/waybar-style.css /usr/share/$(BINARY)/waybar-style.css
	@echo "Installed $(BINARY) v$(VERSION) to /usr/bin/$(BINARY)"

uninstall:
	@echo "Removing $(BINARY)..."
	@rm -f /usr/bin/$(BINARY)
	@rm -f /usr/bin/wizado-menu-float
	@rm -rf /usr/share/$(BINARY)
	@echo "Uninstall complete"

reinstall: clean install

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY)
	rm -f $(BINARY)-linux-amd64
	rm -f *.pkg.tar.zst
	rm -rf pkg/ src/
	@echo "Clean complete"

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

