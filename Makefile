.PHONY: help dev build test clean docker-up docker-down setup env migrate backfill

# .env があれば読み込み、ホストで動かすコマンド（api-dev / bot-dev / migrate）に渡す
-include .env
export

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Start development environment
	@echo "Starting development environment..."
	docker compose up -d mysql redis api web
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
	@echo "Running Bot tests..."
	cd apps/bot && go test ./...
	@echo "Running Core tests..."
	cd packages/core && go test ./...
	@echo "Running Web checks..."
	cd apps/web && npm run type-check

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf apps/api/bin
	rm -rf apps/bot/bin
	rm -rf apps/web/.next
	rm -rf apps/web/node_modules

docker-up: ## Start Docker services
	docker compose up -d

docker-down: ## Stop Docker services
	docker compose down

docker-logs: ## Show Docker logs
	docker compose logs -f

api-dev: ## Start API in development mode
	@echo "Starting API server..."
	cd apps/api && go run main.go

backfill: ## Download historical bars from GMO FX (ARGS="-from 2023-10-28 -interval 1h")
	cd apps/bot && go run . backfill $(ARGS)

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
	cd packages/core && go mod tidy
	@echo "Installing Node.js dependencies..."
	cd apps/web && npm install

env: ## Create .env from .env.example (generates NEXTAUTH_SECRET)
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		sed -i.bak "s|^NEXTAUTH_SECRET=$$|NEXTAUTH_SECRET=$$(openssl rand -base64 32)|" .env && rm -f .env.bak; \
		echo "Created .env (edit DB passwords if needed)"; \
	fi

migrate: ## Apply pending database migrations (host-run, uses .env)
	cd apps/api && go run . migrate

db-migrate: migrate ## Alias of migrate

reload: ## Reload database with migrations and seed data
	docker compose down -v
	docker compose up -d

db-reset: ## Reset database
	$(MAKE) reload

format: ## Format code
	@echo "Formatting Go code..."
	cd apps/api && go fmt ./...
	cd apps/bot && go fmt ./...
	cd packages/core && go fmt ./...
	@echo "Formatting TypeScript code..."
	cd apps/web && npm run lint:fix

setup: ## Environment setup (install deps, start services, optionally build apps)
	@echo "🚀 Starting environment setup..."
	$(MAKE) env
	@echo "📦 Installing dependencies..."
	$(MAKE) install-deps
	@echo "🐳 Starting Docker services..."
	$(MAKE) docker-up
	@echo "⏳ Waiting for services to be ready..."
	@sleep 5
ifdef BUILD
	@echo "🔨 Building applications..."
	$(MAKE) build
	@echo "✅ Complete environment setup completed!"
else
	@echo "✅ Quick environment setup completed!"
endif
	@echo ""
	@echo "🌐 Services available at:"
	@echo "   API: http://localhost:8081"
	@echo "   Web: http://localhost:3001"
	@echo ""
	@echo "📝 Next steps:"
	@echo "   - Run 'make dev' to start all services in development mode"
	@echo "   - Run 'make api-dev' to start API in development mode"
	@echo "   - Run 'make web-dev' to start Web in development mode"
	@echo "   - Run 'make bot-dev' to start Bot in development mode"
	@echo ""
	@echo "💡 Usage:"
	@echo "   make setup          # Quick setup (no build)"
	@echo "   make setup BUILD=1  # Complete setup with build"
