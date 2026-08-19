.PHONY: all test test-integration test-load docker-up docker-down clean build help

GO ?= go
K6 ?= k6
GATEWAY_URL ?= http://localhost:8080

help:
	@echo "دستورات دسترس‌پذیر در JanusGate:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build:
	@echo "==> Building JanusGate binary..."
	$(GO) build -o bin/janusgate ./cmd/gateway

test:
	@echo "==> Running Unit Tests..."
	$(GO) test -v -race ./internal/...

test-integration:
	@echo "==> Running Automated Integration Tests..."
	$(GO) test -v -race -timeout 5m ./tests/integration/...

docker-up:
	@echo "==> Starting Docker infrastructure..."
	docker-compose up -d

docker-down:
	@echo "==> Stopping Docker infrastructure..."
	docker-compose down -v

test-load:
	@echo "==> Running Load Test with k6..."
	GATEWAY_URL=$(GATEWAY_URL) $(K6) run tests/load/k6_test.js

test-all: test test-integration

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin/