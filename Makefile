.PHONY: sdk-test sdk-build build test test-all test-race test-pg lint format format-check run-dev clean docker-up docker-down

export PATH := /home/gin/go/bin:$(PATH)
export GOPATH := /home/gin/go/path

# Every module in go.work, so new modules are picked up automatically
GO_MODULES := $(shell go list -m -f '{{.Dir}}' 2>/dev/null | grep -v '^$(CURDIR)$$')

## Kernel API binary
build:
	cd services/kernel-api && go build -o ../../build/kernel-api ./cmd/kernel-api

## Kernel runtime tests only
test:
	cd packages/kernel-runtime && go test ./... -count=1

## All Go tests in the workspace
test-all:
	@set -e; for d in $(GO_MODULES); do echo "== $$d"; (cd $$d && go test ./... -count=1); done

## All Go tests with the race detector
test-race:
	@set -e; for d in $(GO_MODULES); do echo "== $$d"; (cd $$d && go test -race ./... -count=1); done

## PostgreSQL store tests against a throwaway container (removed afterwards)
test-pg:
	@docker run -d --rm --name omniconnector-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=omni_test \
		-p 127.0.0.1:55439:5432 postgres:16-alpine >/dev/null
	@until docker exec omniconnector-test-pg pg_isready -U postgres -d omni_test >/dev/null 2>&1; do sleep 1; done; sleep 1
	@cd packages/kernel-runtime && KERNEL_TEST_DATABASE_DSN="postgres://postgres:test@127.0.0.1:55439/omni_test?sslmode=disable" \
		go test -race -count=1 ./pgstore/... ; status=$$?; docker rm -f omniconnector-test-pg >/dev/null; exit $$status

## Client SDK: unit and end-to-end tests against the freshly built API
sdk-test: build
	cd packages/client-sdk && npm ci --no-audit --no-fund && npm run typecheck && \
		OMNICONNECTOR_API_BIN=$(CURDIR)/build/kernel-api npm test

## Client SDK: compile to packages/client-sdk/dist
sdk-build:
	cd packages/client-sdk && npm run build

## Vet every module
lint:
	@set -e; for d in $(GO_MODULES); do (cd $$d && go vet ./...); done

## Format every module in place
format:
	gofmt -w $(GO_MODULES)

## Fail if any file is not gofmt-formatted
format-check:
	@out=$$(gofmt -l $(GO_MODULES)); if [ -n "$$out" ]; then echo "unformatted:"; echo "$$out"; exit 1; fi

## Run the API locally with the test integration and a throwaway dev token
run-dev: build
	KERNEL_DEV_TOKEN=dev-token-change-me-0123456789abcdef \
	KERNEL_DEV_PERMISSIONS=admin \
	KERNEL_ENABLE_TEST_INTEGRATION=true \
	./build/kernel-api

## Docker
docker-up:
	docker compose -f deployment/docker-compose.yml up -d --build --wait

docker-down:
	docker compose -f deployment/docker-compose.yml down

## Clean
clean:
	rm -rf build/
