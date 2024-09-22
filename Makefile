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
.phony: dev.deps
dev.deps:
	go install github.com/air-verse/air@latest
	@mkdir -p tmp
	@which air &>/dev/null || echo "air not found in PATH, you may need to add your go bin directory to your PATH"

.phony: dev
## Serve the site
dev:
	@air
	
# --- ci ----------------------------------------------------------------------
.phony: lint
## Runs linting over project
lint:
	npx prettier --check .

.phony: fmt
## Runs formatting over project
fmt:
	npx prettier --write .

# --- cd ----------------------------------------------------------------------
export SHA ?= $(shell git rev-parse --short HEAD)
export VERSION ?= latest
export REGISTRY ?= registry.oliverlambson.com.localhost
.phony: build
## build the image for VERSION. e.g., make build VERSION=1.0
build:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.build.yaml \
		build

.phony: push
## Push the image to the registry
push: build
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.build.yaml \
		push

.phony: save
## Save the image tarball
save: build
	docker save $$REGISTRY/personal-site:$$VERSION > personal-site-latest.tar

.phony: deploy
## Save the image tarball
sync-image: save
	scp personal-site-latest.tar ollie@oliverlambson.com:~/personal-site-latest.tar
	ssh ollie@oliverlambson.com 'docker load < personal-site-latest.tar && rm personal-site-latest.tar'

.phony: deploy
## Deploy application to server
deploy:
	ssh ollie@oliverlambson.com 'rm -rf ~/personal-site'
	scp -r deployment/ ollie@oliverlambson.com:~/personal-site
	ssh ollie@oliverlambson.com 'bash -c "./secrets.sh ~/personal-site/ && ./generate-config.sh -f ~/personal-site/compose.yaml | docker stack deploy -d -c - personal-site"'

# --- docker compose -----------------------------------------------------------
.phony: up
## Run docker compose
up:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.build.yaml \
		--file deployment/compose.dev.yaml \
		up --build --detach
	@echo "dev at: http://localhost:1960"

.phony: down
## Stop docker compose
down:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.build.yaml \
		--file deployment/compose.dev.yaml \
	down --volumes --remove-orphans

.phony: logs
## Follow docker compose logs
logs:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.build.yaml \
		--file deployment/compose.dev.yaml \
	logs -f

# --- docker swarm -------------------------------------------------------------
.phony: swarm
## Run docker swarm stack
swarm: build
	docker stack deploy \
		--detach \
		--compose-file deployment/compose.yaml \
		personal-site

.phony: swarm.stop
## Stop docker swarm stack
swarm.stop:
	docker stack rm personal-site
