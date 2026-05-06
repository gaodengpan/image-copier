NAME=image-copier
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
           -X github.com/gaodengpan/image-copier/internal/version.Version=$(VERSION) \
           -X github.com/gaodengpan/image-copier/internal/version.Commit=$(COMMIT) \
           -X github.com/gaodengpan/image-copier/internal/version.Date=$(DATE)

.PHONY: build clean install uninstall test test-coverage test-coverage-html help fmt vet lint check-quality

build: ## Build the binary
	go build -ldflags "$(LDFLAGS)" -o ${NAME} ./cmd/${NAME}

test: ## Run all tests
	go test -v ./...

test-ci: ## Run tests with race detection (CI mode)
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep total

test-coverage: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep total
	@echo ""
	@echo "Coverage report saved to coverage.out"

test-coverage-html: ## Generate HTML coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report saved to coverage.html"
	@echo "Open coverage.html in your browser to view the report"

install: ## Install the binary to GOBIN (default: $GOPATH/bin)
	go install -ldflags "$(LDFLAGS)" ./cmd/${NAME}

uninstall: ## Uninstall the binary from GOBIN (default: $GOPATH/bin)
	rm -f $(shell go env GOPATH)/bin/${NAME}

clean: ## Clean build artifacts
	rm -f ${NAME}

fmt: ## Format code with gofmt and goimports
	gofmt -w .
	goimports -w .

vet: ## Run go vet on the code
	go vet ./...

lint: ## Run golangci-lint (requires installation: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

check-quality: fmt vet lint test-ci ## Run comprehensive quality checks (formatting, vet, lint, and tests with race detection)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
