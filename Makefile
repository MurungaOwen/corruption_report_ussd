.PHONY: run build test vet seed docker-build docker-up docker-down fmt e2e

run: ## Run the server locally (uses .env if present via `set -a`)
	@[ -f .env ] && set -a && . ./.env && set +a; go run ./cmd/server

build: ## Build static binaries into ./bin
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/seed ./cmd/seed

test: ## Run the test suite
	go test ./... -count=1

vet: ## Static analysis
	go vet ./...

fmt: ## Format all Go source
	gofmt -l -w .

seed: ## Populate the local dev database with ~200 demo Kenyan officials
	@[ -f .env ] && set -a && . ./.env && set +a; go run ./cmd/seed

docker-build: ## Build the production container image
	docker build -t ussdgov:local .

docker-up: ## Run via docker-compose (reads .env)
	docker compose up --build

docker-down:
	docker compose down

e2e: ## Run the Playwright UAT suite end to end: build, seed, serve, test, tear down
	@set -e; \
	DIR=$$(mktemp -d); \
	trap 'kill $$SERVER_PID 2>/dev/null; rm -rf $$DIR' EXIT; \
	go build -o $$DIR/server-bin ./cmd/server; \
	go build -o $$DIR/e2eseed-bin ./cmd/e2eseed; \
	DB_PATH=$$DIR/reports.db UPLOADS_DIR=$$DIR/uploads ADDR=:8097 \
		PUBLIC_BASE_URL=http://localhost:8097 ENVIRONMENT=development \
		$$DIR/server-bin > $$DIR/server.log 2>&1 & \
	SERVER_PID=$$!; \
	for i in $$(seq 1 30); do curl -sf http://localhost:8097/healthz >/dev/null 2>&1 && break; sleep 0.5; done; \
	DB_PATH=$$DIR/reports.db UPLOADS_DIR=$$DIR/uploads $$DIR/e2eseed-bin; \
	cd e2e && npm install --no-audit --no-fund && E2E_BASE_URL=http://localhost:8097 npx playwright test
