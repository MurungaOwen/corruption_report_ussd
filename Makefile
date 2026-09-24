.PHONY: run build test vet seed docker-build docker-up docker-down fmt

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
