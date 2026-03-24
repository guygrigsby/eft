.PHONY: wasm web serve clean test

GOROOT_WASM_EXEC := $(shell go env GOROOT)/lib/wasm/wasm_exec.js

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
