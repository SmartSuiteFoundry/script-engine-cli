.PHONY: build test vet cross-linux cross-darwin cross-windows cross clean

# Output for `make build`. Use an explicit path so an empty env (e.g. BINARY=)
# never produces `go build -o ./cmd/sse`, which can mis-place or corrupt outputs
# under ./cmd/sse/. Override: make build SSE_OUTPUT=/usr/local/bin/sse
SSE_OUTPUT ?= $(CURDIR)/sse

build:
	go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o "$(SSE_OUTPUT)" ./cmd/sse

test:
	go test ./...

vet:
	go vet ./...

cross-linux:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o dist/sse-linux-amd64 ./cmd/sse
	GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o dist/sse-linux-arm64 ./cmd/sse

cross-darwin:
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o dist/sse-darwin-amd64 ./cmd/sse
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o dist/sse-darwin-arm64 ./cmd/sse

cross-windows:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o dist/sse-windows-amd64.exe ./cmd/sse
	GOOS=windows GOARCH=arm64 go build -ldflags "-s -w -X main.Version=$${VERSION:-dev}" -o dist/sse-windows-arm64.exe ./cmd/sse

cross: cross-linux cross-darwin cross-windows

# Versioned tar.gz + SHA256SUMS under dist/ (for customer CDN / install.sh).
# Example: VERSION=1.0.0 make dist-archives
dist-archives:
	VERSION="$${VERSION:-0.0.0-dev}" ./scripts/dist-archives.sh

clean:
	rm -f "$(SSE_OUTPUT)" sse
	rm -rf dist/
