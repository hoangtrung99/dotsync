# Dotsync Makefile

APP_NAME := dotsync
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

.PHONY: all build build-dev install clean test coverage lint run help

all: build

## Build optimized binary
build:
	go build -ldflags="$(LDFLAGS)" -o $(APP_NAME) .

## Build with debug info
build-dev:
	go build -o $(APP_NAME) .

## Install to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	cp $(APP_NAME) ~/.local/bin/
	@echo "Installed to ~/.local/bin/$(APP_NAME)"

## Remove build artifacts
clean:
	rm -f $(APP_NAME)
	go clean

## Run tests
test:
	go test -v ./...

## Run tests with coverage
test-coverage: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Run linter
lint:
	golangci-lint run ./...

## Run the application
run: build
	./$(APP_NAME)

## Update dependencies
deps:
	go mod download
	go mod tidy

## Show help
help:
	@echo "Dotsync - Dotfiles Sync Tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build         Build optimized binary"
	@echo "  make build-dev     Build with debug info"
	@echo "  make install       Install to ~/.local/bin"
	@echo "  make clean         Remove build artifacts"
	@echo "  make test          Run tests"
	@echo "  make test-coverage Run tests with coverage report"
	@echo "  make lint          Run linter"
	@echo "  make run           Build and run"
	@echo "  make deps          Update dependencies"
	@echo "  make help          Show this help"
