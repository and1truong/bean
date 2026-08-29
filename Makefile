SHELL := /bin/bash
GOCACHE := $(CURDIR)/.cache/go-build
export GOCACHE

.PHONY: bootstrap fmt fmt-check lint test test-integration test-e2e check build run clean
bootstrap:
	go mod download
	cd web && bun install --frozen-lockfile
	cd e2e && bun install --frozen-lockfile

fmt:
	gofmt -w cmd examples internal

fmt-check:
	@test -z "$$(gofmt -l cmd examples internal)"

lint:
	go vet ./...
	cd web && bun run lint
	cd web && bun run typecheck

test:
	go test ./...
	cd web && bun run test

test-integration:
	go test ./internal/action ./internal/release ./internal/view

test-e2e: build
	cd e2e && bunx playwright test

check: fmt-check lint test test-integration
	go test -race ./...
	$(MAKE) test-e2e

build:
	cd web && bun run build
	mkdir -p bin
	go build -trimpath -ldflags "-s -w" -o bin/bean ./cmd/bean

run: build
	./bin/bean serve --db ./bean.db --addr 127.0.0.1:8080

clean:
	trash bin .cache 2>/dev/null || true
