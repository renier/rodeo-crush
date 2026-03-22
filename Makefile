BINARY  := rodeo
MODULE  := github.com/renier/rodeo-crush
CMD     := ./cmd/rodeo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.DEFAULT_GOAL := help

##@ Build

.PHONY: build
build: ## Build the rodeo binary
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

.PHONY: install
install: ## Install rodeo to $GOPATH/bin
	go install $(LDFLAGS) $(CMD)

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY)
	go clean -cache -testcache

##@ Development

.PHONY: test
test: ## Run all tests
	go test ./... -count=1

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	go test ./... -v -count=1

.PHONY: test-race
test-race: ## Run tests with race detector
	go test ./... -race -count=1

.PHONY: cover
cover: ## Run tests with coverage and open report
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format all Go source files
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check formatting (fails if files need formatting)
	@test -z "$$(gofmt -l .)" || { echo "Files need formatting:"; gofmt -l .; exit 1; }

.PHONY: lint
lint: vet fmt-check ## Run all linters (vet + format check)

.PHONY: tidy
tidy: ## Tidy and verify go.mod
	go mod tidy
	go mod verify

##@ CI

.PHONY: ci
ci: lint test-race build ## Run full CI pipeline (lint, race tests, build)

##@ Help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } \
		/^[a-zA-Z_-]+:.*##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
