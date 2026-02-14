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

install: ## Install the binary to /usr/local/bin
	go build -ldflags "$(LDFLAGS)" -o ${NAME} ./cmd/${NAME}
	sudo mv ${NAME} /usr/local/bin/

uninstall: ## Uninstall the binary from /usr/local/bin
	sudo rm -f /usr/local/bin/${NAME}

clean: ## Clean build artifacts
	rm -f ${NAME}

fmt: ## Format code with gofmt and goimports
	gofmt -w .
	goimports -w .

vet: ## Run go vet on the code
	go vet ./...

lint: ## Run golangci-lint (requires installation: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

check-quality: fmt vet lint test-coverage ## Run comprehensive quality checks (formatting, vet, lint, and test coverage)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
