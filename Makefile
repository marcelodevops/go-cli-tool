.PHONY: build cross build-macos-arm64 build-linux-amd64 test fmt vet

BINARY_NAME = cli-tool
PKG = github.com/marcelodevops/cli-tool

build:
	go build -o $(BINARY_NAME) ./cmd/cli-tool

cross:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 ./cmd/cli-tool
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 ./cmd/cli-tool

build-macos-arm64:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 ./cmd/cli-tool

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 ./cmd/cli-tool

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...
