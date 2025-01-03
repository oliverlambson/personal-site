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
	npx prettier --check .

.PHONY: fmt
## Runs formatting over project
fmt:
	npx prettier --write .

# --- cd ----------------------------------------------------------------------
export SHA ?= $(shell git rev-parse --short HEAD)
export VERSION ?= latest
export REGISTRY ?= registry.oliverlambson.com

.PHONY: login
## login to docker registry
login:
	op item get --vault oliverlambson.com docker-registry-user --field password --reveal | docker login -u docker-registry-user --password-stdin registry.oliverlambson.com

.PHONY: build
## build the image for VERSION. e.g., make build VERSION=1.0
build:
	docker buildx build \
		--file deployment/Dockerfile \
		--tag "${REGISTRY}/personal-site:${SHA}" \
		--tag "${REGISTRY}/personal-site:${VERSION}" \
		--build-arg VERSION="${VERSION}" \
		--build-arg SHA="${SHA}" \
		--platform linux/arm64 \
		.

.PHONY: push
## Push the image to the registry
push: build login
	docker push "${REGISTRY}/personal-site:${SHA}"
	docker push "${REGISTRY}/personal-site:${VERSION}"

.PHONY: deploy
## Deploy application to server
deploy: push
	rsync \
		-av \
		-e ssh \
		--exclude='.env' \
		--exclude-from="deployment/.rsyncignore" \
		deployment/ \
		ollie@oliverlambson.com:~/stacks/personal-site/
	ssh ollie@oliverlambson.com "bash -c 'deploy-stack personal-site'"

# --- docker compose -----------------------------------------------------------
.PHONY: up
## Run docker compose
up:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.dev.yaml \
		up --build --detach
	@echo "dev at: http://localhost:1960"

.PHONY: down
## Stop docker compose
down:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.dev.yaml \
	down --volumes --remove-orphans

.PHONY: logs
## Follow docker compose logs
logs:
	docker compose \
		--file deployment/compose.yaml \
		--file deployment/compose.dev.yaml \
	logs -f

# --- docker swarm -------------------------------------------------------------
.PHONY: swarm
## Run docker swarm stack
swarm: build
	docker stack deploy \
		--detach \
		--compose-file deployment/compose.yaml \
		personal-site

.PHONY: swarm.stop
## Stop docker swarm stack
swarm.stop:
	docker stack rm personal-site
