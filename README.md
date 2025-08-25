# Moomoo Trading System

Advanced algorithmic trading platform built with Next.js, Go, and MySQL.

## Architecture

This is a monorepo containing:

- `apps/web` - Next.js frontend application
- `apps/api` - Go REST API server
- `apps/bot` - Go worker for strategy execution
- `packages/shared` - Shared utilities and types

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+
- Node.js 18+
- MySQL 8.0
- Redis 7.0

### Development Setup

1. **Start Infrastructure**
   ```bash
   docker-compose up -d
   ```

2. **Start API Server**
   ```bash
   cd apps/api
   go run main.go
   ```

3. **Start Web Application**
   ```bash
   cd apps/web
   npm run dev
   ```

4. **Start Bot Worker**
   ```bash
   cd apps/bot
   go run main.go
   ```

## Features

### Phase 1: Foundation (In Progress)
- [x] Monorepo structure
- [x] Docker development environment
- [x] MySQL database schema
- [x] Basic API endpoints
- [x] Next.js frontend setup
- [x] Go worker framework

### Phase 2: Core Features (Planned)
- [ ] Moomoo broker adapter
- [ ] Strategy engine with Starlark
- [ ] Risk management system
- [ ] Backtesting engine

### Phase 3: Advanced Features (Planned)
- [ ] Real-time monitoring
- [ ] Paper trading workflow
- [ ] Performance analytics
- [ ] Notification system

## API Endpoints

- `GET /healthz` - Health check
- `GET /api/v1/orders` - List orders
- `POST /api/v1/orders` - Create order
- `GET /api/v1/strategies` - List strategies
- `POST /api/v1/strategies` - Create strategy
- `GET /api/v1/backtests` - List backtests
- `POST /api/v1/backtests` - Create backtest

## Environment Variables

### Database
- `DB_HOST` - MySQL host (default: localhost)
- `DB_PORT` - MySQL port (default: 3306)
- `DB_USER` - MySQL user (default: moomoo)
- `DB_PASSWORD` - MySQL password (default: moomoo123)
- `DB_NAME` - MySQL database (default: moomoo_trading)

### Redis
- `REDIS_HOST` - Redis host (default: localhost)
- `REDIS_PORT` - Redis port (default: 6379)
- `REDIS_PASSWORD` - Redis password (default: empty)

### Moomoo
- `MOOMOO_HOST` - Moomoo OpenD host
- `MOOMOO_USERNAME` - Moomoo username
- `MOOMOO_PASSWORD` - Moomoo password
- `MOOMOO_APP_ID` - Moomoo app ID
- `MOOMOO_APP_KEY` - Moomoo app key

## Development

### Database Migrations

Database schema is automatically initialized when the MySQL container starts. See `scripts/init.sql` for the current schema.

### Adding New API Endpoints

1. Add handler function in `apps/api/internal/handlers/`
2. Register route in `apps/api/main.go`
3. Add corresponding frontend components in `apps/web/src/`

### Strategy Development

Strategies are written in Starlark and stored in the database. The bot worker executes strategies based on market events.

## License

MIT License
