.PHONY: build test-unit test-coverage test-integration test-e2e run vet lint swagger k6-smoke k6-load k6-spike k6-soak k6-all k6-compare

VERSION ?= dev
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:
	go build -ldflags="\
	  -X github.com/finish06/drug-gate/internal/version.Version=$(VERSION) \
	  -X github.com/finish06/drug-gate/internal/version.BuildTime=$(BUILD_TIME)" \
	  -o bin/server ./cmd/server

run:
	go run ./cmd/server

test-unit:
	go test ./... -short -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-integration:
	@echo "Starting Redis..."
	@docker compose up -d redis
	@echo "Waiting for Redis to be ready..."
	@until docker compose exec redis redis-cli ping 2>/dev/null | grep -q PONG; do sleep 0.5; done
	@echo "Running integration tests..."
	REDIS_URL=localhost:6379 go test -tags=integration -count=1 -v ./...; \
	EXIT_CODE=$$?; \
	docker compose stop redis; \
	exit $$EXIT_CODE

test-e2e:
	@echo "Building and starting e2e stack (drug-gate + Redis + cash-drugs + MongoDB)..."
	@docker compose -f docker-compose.e2e.yml up -d --build
	@echo "Waiting for cash-drugs to be ready..."
	@until docker compose -f docker-compose.e2e.yml exec cash-drugs wget -q --spider http://localhost:8080/health 2>/dev/null; do sleep 1; done
	@echo "Waiting for drug-gate to be ready..."
	@until curl -sf http://localhost:18081/health >/dev/null 2>&1; do sleep 1; done
	@echo "Running e2e tests..."
	DRUG_GATE_URL=http://localhost:18081 ADMIN_SECRET=e2e-test-secret \
		go test -tags=e2e -count=1 -v ./tests/e2e/...; \
	EXIT_CODE=$$?; \
	echo "Tearing down e2e stack..."; \
	docker compose -f docker-compose.e2e.yml down; \
	exit $$EXIT_CODE

vet:
	go vet ./...

lint:
	golangci-lint run ./...

swagger:
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

# k6 performance tests (staging)
K6_BASE_URL ?= http://192.168.1.145:8082
K6_API_KEY  ?= pk_1bf389dc3ef894d25f1fee1c4797a3eef371b4eec6d17a02

k6-smoke:
	k6 run tests/k6/staging.js --env SCENARIO=smoke --env BASE_URL=$(K6_BASE_URL) --env API_KEY=$(K6_API_KEY) --summary-export=/tmp/k6-smoke.json
	@node tests/k6/compare.js smoke /tmp/k6-smoke.json

k6-load:
	k6 run tests/k6/staging.js --env SCENARIO=load --env BASE_URL=$(K6_BASE_URL) --env API_KEY=$(K6_API_KEY) --summary-export=/tmp/k6-load.json
	@node tests/k6/compare.js load /tmp/k6-load.json

k6-spike:
	k6 run tests/k6/staging.js --env SCENARIO=spike --env BASE_URL=$(K6_BASE_URL) --env API_KEY=$(K6_API_KEY) --summary-export=/tmp/k6-spike.json
	@node tests/k6/compare.js spike /tmp/k6-spike.json

k6-soak:
	k6 run tests/k6/staging.js --env SCENARIO=soak --env BASE_URL=$(K6_BASE_URL) --env API_KEY=$(K6_API_KEY) --summary-export=/tmp/k6-soak.json
	@node tests/k6/compare.js soak /tmp/k6-soak.json

k6-all:
	@echo "Running all k6 scenarios with baseline comparison..."
	$(MAKE) k6-smoke
	$(MAKE) k6-load
	$(MAKE) k6-spike
	$(MAKE) k6-soak
