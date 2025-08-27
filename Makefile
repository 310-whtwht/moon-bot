.PHONY: help dev build test clean docker-up docker-down

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Start development environment
	@echo "Starting development environment..."
	docker compose up -d mysql redis web
	@echo "Development environment started!"
	@echo "API: http://localhost:8081"
	@echo "Web: http://localhost:3001"

build: ## Build all applications
	@echo "Building API..."
	cd apps/api && go build -o bin/api main.go
	@echo "Building Bot..."
	cd apps/bot && go build -o bin/bot main.go
	@echo "Building Web..."
	cd apps/web && npm run build

test: ## Run tests
	@echo "Running API tests..."
	cd apps/api && go test ./...
	@echo "Running Web tests..."
	cd apps/web && npm test

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf apps/api/bin
	rm -rf apps/bot/bin
	rm -rf apps/web/.next
	rm -rf apps/web/node_modules

docker-up: ## Start Docker services
	docker-compose up -d

docker-down: ## Stop Docker services
	docker-compose down

docker-logs: ## Show Docker logs
	docker-compose logs -f

api-dev: ## Start API in development mode
	@echo "Starting API server..."
	cd apps/api && go run main.go

web-dev: ## Start Web in development mode
	@echo "Starting Web application..."
	cd apps/web && npm run dev

bot-dev: ## Start Bot in development mode
	@echo "Starting Bot worker..."
	cd apps/bot && go run main.go

install-deps: ## Install all dependencies
	@echo "Installing Go dependencies..."
	cd apps/api && go mod tidy
	cd apps/bot && go mod tidy
	@echo "Installing Node.js dependencies..."
	cd apps/web && npm install

db-migrate: ## Run database migrations
        @echo "Database migrations and seed data are executed on container startup"
        @echo "See apps/api/internal/database/migrations for SQL files"

reload: ## Reload database with migrations and seed data
        docker-compose down -v
        docker-compose up -d

db-reset: ## Reset database
        $(MAKE) reload

format: ## Format code
	@echo "Formatting Go code..."
	cd apps/api && go fmt ./...
	cd apps/bot && go fmt ./...
	@echo "Formatting TypeScript code..."
	cd apps/web && npm run lint:fix
