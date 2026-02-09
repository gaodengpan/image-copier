NAME=image-copier
VERSION=0.1.0

.PHONY: build clean install uninstall help

build: ## Build the binary
	go build -o ${NAME} ./cmd/${NAME}

install: ## Install the binary to /usr/local/bin
	go build -o ${NAME} ./cmd/${NAME}
	sudo mv ${NAME} /usr/local/bin/

uninstall: ## Uninstall the binary from /usr/local/bin
	sudo rm -f /usr/local/bin/${NAME}

clean: ## Clean build artifacts
	rm -f ${NAME}

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
