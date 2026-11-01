# Ol1n Pass — build, test, sync and deploy. Paths and names follow docs/CONTRACTS.md.
.DEFAULT_GOAL := help

OLIN_NOW ?= ../ol1n.now
IMAGE    ?= ghcr.io/lioilsources/pass-api
TAG      ?= latest
GOFLAGS  ?= -trimpath
export CGO_ENABLED = 0

.PHONY: help test test-go test-flutter build check-catalog sync-catalog sync-site docker deploy-pass-api clean

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

test: test-go test-flutter ## Go + Flutter tests

test-go: ## go test ./...
	go test ./...

test-flutter: ## flutter test in ol1n_pass/
	cd ol1n_pass && flutter test

build: ## Static Go binaries → bin/
	mkdir -p bin
	go build $(GOFLAGS) -ldflags='-s -w' -o bin/pass-api ./cmd/pass-api
	go build $(GOFLAGS) -ldflags='-s -w' -o bin/passctl ./cmd/passctl

check-catalog: ## Validate catalog/apps.json (schema, unique slugs, value total)
	go run ./cmd/passctl catalog check

sync-catalog: ## Copy catalog/apps.json to the package asset and the embedded Go default
	cp catalog/apps.json ol1n_pass/assets/apps.json
	cp catalog/apps.json internal/catalog/default_apps.json

sync-site: ## Copy site/ + catalog into the ol1n.now checkout ($(OLIN_NOW)); commit + deploy there
	@test -d "$(OLIN_NOW)/pages" || { echo "ol1n.now checkout not found at $(OLIN_NOW) (set OLIN_NOW=…)"; exit 1; }
	mkdir -p "$(OLIN_NOW)/static" "$(OLIN_NOW)/assets/css"
	cp site/pass.html    "$(OLIN_NOW)/pages/pass.html"
	cp site/pass.en.html "$(OLIN_NOW)/pages/pass.en.html"
	cp site/pass.css     "$(OLIN_NOW)/assets/css/pass.css"
	cp catalog/apps.json "$(OLIN_NOW)/static/pass.json"
	@echo "synced → $(OLIN_NOW); now: cd $(OLIN_NOW) && python3 scripts/check-page-translation.py pages/pass.html pages/pass.en.html && make build"

docker: ## Build the pass-api image for linux/amd64 (JODA)
	docker build --platform linux/amd64 -t $(IMAGE):$(TAG) -f deploy/Dockerfile .

deploy-pass-api: docker ## Push the image and restart pass-api on JODA (ssh alias `joda`)
	docker push $(IMAGE):$(TAG)
	ssh joda 'cd ~/deploy/pass-api && docker compose pull && docker compose up -d && docker compose ps'

clean: ## Remove build output
	rm -rf bin
