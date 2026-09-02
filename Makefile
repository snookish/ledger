.DEFAULT_GOAL := help

.PHONY: help build run test test-unit cover bench demo lint fmt tidy vet clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o bin/$(notdir $(CURDIR)) ./...

run: ## Run the application
	go run ./...

test: ## Run all tests with race detector
	go test -v -race ./...

test-unit: ## Run tests without race detector
	go test -v ./...

cover: ## Run tests and print coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "coverage report: coverage.html"

bench: ## Run all benchmarks
	go test -bench . -benchmem ./...

demo: ## Build and run demo (via scripts/run.sh)
	./scripts/run.sh

lint: ## Run golangci-lint
	golangci-lint run ./... 2>/dev/null || echo "hint: mise use golangci-lint"

fmt: ## Format code (gofmt + goimports)
	golangci-lint fmt ./... 2>/dev/null || gofmt -w -s .

tidy: ## Tidy modules
	go mod tidy

vet: ## Run go vet
	go vet ./...

clean: ## Clean artifacts and coverage files
	rm -rf bin/ coverage.out coverage.html

examples: ## Run examples
	./run.sh
