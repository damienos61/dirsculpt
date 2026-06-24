.PHONY: build run clean test deps build-all

BINARY=dirsculpt
VERSION=$(shell date +%Y%m%d)

deps:
	go mod download
	go mod tidy

build:
	go build -ldflags="-s -w" -o $(BINARY)

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dirsculpt-linux

build-macos:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dirsculpt-macos

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dirsculpt.exe

build-all: build-linux build-macos build-windows
	@echo "✅ All binaries built"

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) dirsculpt-*
	find . -name "*.out" -delete

test:
	go test -v ./...
