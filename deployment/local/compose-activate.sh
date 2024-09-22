#!/usr/env/bin bash

export COMPOSE_FILE=$(find deployment -type f -name 'compose*.yaml' | tr '\n' ':' | sed 's/:$//')
export SHA=$(git rev-parse --short HEAD)
export VERSION=latest
