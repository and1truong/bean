SHELL := /bin/bash
GOCACHE := $(CURDIR)/.cache/go-build
export GOCACHE

.PHONY: bootstrap fmt fmt-check lint test test-integration test-contract test-fuzz-smoke test-compatibility test-blackbox test-crash qualify-durability test-blog test-postgres test-e2e check build run clean
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

test-contract:
	go test ./internal/compiler ./internal/action ./internal/view ./internal/dbal/sqlite ./internal/migration ./internal/policy ./internal/webform ./internal/render

test-fuzz-smoke:
	go test ./internal/definition ./internal/expr ./internal/view ./internal/httpapi -run '^Fuzz'

test-compatibility:
	go test ./internal/appir ./internal/release -run 'Compatibility|Format'

test-blackbox: build
	@test "$$($(CURDIR)/bin/bean version)" = "bean 0.13.0-alpha"
	BEAN_BINARY=$(CURDIR)/bin/bean go test ./internal/agenttest -count=1

test-crash: build
	BEAN_BINARY=$(CURDIR)/bin/bean go test ./internal/crashtest -count=1

qualify-durability: build
	BEAN_BINARY=$(CURDIR)/bin/bean go test ./internal/crashtest -count=20

test-postgres: build
	@set -e; docker rm -f bean-postgres-test >/dev/null 2>&1 || true; \
	docker run --rm -d --name bean-postgres-test -e POSTGRES_PASSWORD=bean -e POSTGRES_DB=bean -p 127.0.0.1:55432:5432 postgres:17-alpine >/dev/null; \
	trap 'docker stop bean-postgres-test >/dev/null 2>&1 || true' EXIT; \
	for attempt in $$(seq 1 30); do docker logs bean-postgres-test 2>&1 | grep -q 'PostgreSQL init process complete; ready for start up.' && docker exec bean-postgres-test pg_isready -U postgres -d bean >/dev/null 2>&1 && break; sleep 1; done; \
	docker exec bean-postgres-test pg_isready -U postgres -d bean >/dev/null; \
	docker exec bean-postgres-test createdb -U postgres bean_blog; \
	docker exec bean-postgres-test createdb -U postgres bean_blog_e2e; \
	docker exec bean-postgres-test createdb -U postgres bean_agent; \
	BEAN_TEST_POSTGRES_URL=postgres://postgres:bean@127.0.0.1:55432/bean?sslmode=disable BEAN_TEST_BLOG_POSTGRES_URL=postgres://postgres:bean@127.0.0.1:55432/bean_blog?sslmode=disable BEAN_TEST_AGENT_POSTGRES_URL=postgres://postgres:bean@127.0.0.1:55432/bean_agent?sslmode=disable go test ./internal/dbal/postgres ./internal/httpapi ./internal/agentprotocol -count=1; \
	cd e2e && BEAN_E2E_DATABASE_URL=postgres://postgres:bean@127.0.0.1:55432/bean_blog_e2e?sslmode=disable bunx playwright test blog.spec.ts

test-e2e: build
	cd e2e && bunx playwright test

test-blog: build
	cd e2e && bunx playwright test blog.spec.ts

check: fmt-check lint test test-integration test-contract test-fuzz-smoke test-compatibility test-blackbox
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
