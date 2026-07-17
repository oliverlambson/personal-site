.DEFAULT_GOAL := help

YELLOW := \033[0;33m
RESET := \033[0m

.PHONY: help
## Prints this help
help:
	@echo "\nUsage: make ${YELLOW}[arg=value] <target>${RESET}\n\nThe following targets are available:\n";
	@awk -v skip=1 \
		'/^##/ { sub(/^[#[:blank:]]*/, "", $$0); doc_h=$$0; doc=""; skip=0; next } \
		 skip  { next } \
		 /^#/  { doc=doc "\n" substr($$0, 2); next } \
		 /:/   { sub(/:.*/, "", $$0); printf "\033[34m%-30s\033[0m\033[1m%s\033[0m %s\n", $$0, doc_h, doc; skip=1 }' \
		$(MAKEFILE_LIST)

# --- develop ------------------------------------------------------------------
.PHONY: dev.deps
dev.deps:
	go install github.com/air-verse/air@latest
	@mkdir -p tmp
	@which air &>/dev/null || echo "air not found in PATH, you may need to add your go bin directory to your PATH"

.PHONY: dev
## Serve the site
dev:
	@air
	
# --- ci ----------------------------------------------------------------------
.PHONY: lint
## Runs linting over project
lint:
	npx --no-install prettier --check .

.PHONY: fmt
## Runs formatting over project
fmt:
	npx --no-install prettier --write .

.PHONY: test
## Runs the test suite
test:
	go test ./...

.PHONY: build
## Builds the static personal-site binary
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/personal-site ./cmd
