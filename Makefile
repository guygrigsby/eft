.PHONY: build wasm web serve clean test release

GOROOT_WASM_EXEC := $(shell go env GOROOT)/lib/wasm/wasm_exec.js

# Build the native CLI binary.
build:
	go build -o eft ./cmd/eft/

# Build the WASM binary.
wasm:
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o web/eft.wasm ./cmd/wasm/

# Copy the Go WASM support JS (version-matched to the Go compiler).
web/wasm_exec.js:
	cp "$(GOROOT_WASM_EXEC)" web/wasm_exec.js

# Build all web assets.
web: wasm web/wasm_exec.js

# Start a local dev server.
serve: web
	@echo "Serving at http://localhost:8080"
	cd web && python3 -m http.server 8080

# Run all tests.
test:
	go test ./...

# Remove built artifacts.
clean:
	rm -f web/eft.wasm web/wasm_exec.js

VERSION ?= v1.0.0

# Create a GitHub release with cross-compiled CLI binaries.
release: clean test
	@echo "Building release $(VERSION)..."
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o dist/eft-linux-amd64       ./cmd/eft/
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o dist/eft-linux-arm64       ./cmd/eft/
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o dist/eft-darwin-amd64      ./cmd/eft/
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o dist/eft-darwin-arm64      ./cmd/eft/
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/eft-windows-amd64.exe ./cmd/eft/
	gh release create $(VERSION) dist/* --title "$(VERSION)" --generate-notes
