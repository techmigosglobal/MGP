SHELL := /bin/sh

APP := mgp-server
TEMPL_VERSION := v0.3.1020

.PHONY: generate assets dev test lint build docker

generate:
	go run github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION) generate

assets:
	npm --prefix web ci
	npm --prefix web run build

dev: generate assets
	go run ./cmd/mgp-server

test:
	go test ./...

lint:
	test -z "$$(gofmt -l $$(find . -name '*.go' -type f))"
	go vet ./...

build: generate assets
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/$(APP) ./cmd/mgp-server

docker:
	docker build -f deploy/Dockerfile -t mgp:local .
