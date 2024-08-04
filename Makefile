.DEFAULT_GOAL := help

YELLOW := \033[0;33m
RESET := \033[0m

.phony: help
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
.phony: serve
## Serve the site
serve:
	@echo "${YELLOW}TODO: still using python${RESET}"
	python -m http.server -d web/static

# --- build --------------------------------------------------------------------
.phony: build
## Builds the deployable site
build:
	$(MAKE) build.content

.phony: build.content
## Builds html snippets from markdown content
build.content:
	scripts/build/pandoc.sh

# --- ci ----------------------------------------------------------------------
.phony: lint
## Runs linting over project
lint:
	scripts/ci/lint.sh

.phony: fmt
## Runs formatting over project
fmt:
	scripts/ci/fmt.sh
