.PHONY: \
	image \
	up \
	docker-compose \
	seed \
	migrate \
	test \
	\

.DEFAULT_GOAL:=help

SHELL := /bin/bash

# ==============================================================================
# Building containers

# $(shell git rev-parse --short HEAD)
VERSION := dev
DOCKER_COMPOSE_FILE := infra/docker-compose.yml

image:		## Build a shorterer service image in docker
	@:$(call check_defined, VERSION, version)
	docker build \
		-f infra/shortener.Dockerfile \
		-t url-shortener:$(VERSION) \
		--build-arg BUILD_REF=$(VERSION) \
		--build-arg BUILD_DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"` \
		.

up:             ## Build and start shortener service and dependencies
	docker-compose -f $(DOCKER_COMPOSE_FILE) up --build -d --force-recreate --remove-orphans shortener

# ==============================================================================
# Docker support

docker-down:
	docker rm -f $(shell docker ps -aq)

docker-clean:
	docker system prune -f

docker-compose: ## Run docker-compose command e.g. '--help'
	@read -p "Enter docker-compose command: " CMD && \
	docker-compose -f $(DOCKER_COMPOSE_FILE) $$CMD

clean:          ## Stops running services and removes containers, volumes and images
	docker-compose -f $(DOCKER_COMPOSE_FILE) kill
	docker-compose -f $(DOCKER_COMPOSE_FILE) rm -sfv

# ==============================================================================
# Administration

migrate:	## Initialize a new database in postgres container
	docker-compose -f $(DOCKER_COMPOSE_FILE) run --rm admin /admin migrate

seed: migrate	## Seeds initial data to the new database
	docker-compose -f $(DOCKER_COMPOSE_FILE) run --rm admin /admin seed

# ==============================================================================
# Running tests within the local computer

test: seed	## Run tests inside a container
	docker-compose -f $(DOCKER_COMPOSE_FILE) run --rm test

# ==============================================================================
# Modules support

deps-reset:
	git checkout -- go.mod
	go mod tidy
	go mod vendor

tidy:
	go mod tidy

deps-upgrade:
	# go get $(go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -m all)
	go get -u -v ./...
	go mod tidy

deps-cleancache:
	go clean -modcache

list:
	go list -mod=mod all

# ==============================================================================
# Help

help:		## Show this help message
	@echo
	@echo '  Usage:'
	@echo '    make <target>'
	@echo
	@echo '  Targets:'
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
	@echo
