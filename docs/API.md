# Moomoo Trading System API Documentation

## Overview

The Moomoo Trading System API provides a RESTful interface for managing trading strategies, orders, and backtesting.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

The API uses NextAuth for authentication. Include the session token in the Authorization header:

```
Authorization: Bearer <session_token>
```

## Endpoints

### Health Check

#### GET /healthz

Returns the health status of the system.

**Response:**
```json
{
  "status": "healthy",
  "service": "moomoo-trading-api",
  "version": "1.0.0",
  "timestamp": 1703123456,
  "uptime": "24h30m15s",
  "system": {
    "goroutines": 15,
    "memory": {
      "alloc": 1024000,
      "total_alloc": 2048000,
      "sys": 4096000,
      "num_gc": 5
    }
  },
  "checks": {
    "database": "healthy",
    "redis": "healthy",
    "broker": "connected"
  },
  "metrics": {
    "lag": 0,
    "queue_size": 0,
    "reconnect_count": 0,
    "vm_quota_hits": 0
  }
}
```

### Orders

#### GET /orders

Retrieves all orders.

**Query Parameters:**
- `status` (optional): Filter by order status
- `symbol` (optional): Filter by symbol
- `limit` (optional): Number of orders to return (default: 100)
- `offset` (optional): Number of orders to skip (default: 0)

**Response:**
```json
{
  "orders": [
    {
      "id": "order_123",
      "client_order_id": "client_123",
      "symbol": "AAPL",
      "side": "BUY",
      "type": "MARKET",
      "quantity": 100,
      "price": null,
      "status": "FILLED",
      "filled_quantity": 100,
      "avg_fill_price": 185.50,
      "commission": 1.00,
      "created_at": "2024-01-15T14:30:00Z",
      "updated_at": "2024-01-15T14:30:05Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

#### POST /orders

Creates a new order.

**Request Body:**
```json
{
  "symbol": "AAPL",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 100,
  "price": null,
  "stop_price": null
}
```

**Response:**
```json
{
  "id": "order_123",
  "client_order_id": "client_123",
  "symbol": "AAPL",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 100,
  "status": "SUBMITTED",
  "created_at": "2024-01-15T14:30:00Z"
}
```

#### GET /orders/{id}

Retrieves a specific order.

**Response:**
```json
{
  "id": "order_123",
  "client_order_id": "client_123",
  "symbol": "AAPL",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 100,
  "status": "FILLED",
  "filled_quantity": 100,
  "avg_fill_price": 185.50,
  "commission": 1.00,
  "created_at": "2024-01-15T14:30:00Z",
  "updated_at": "2024-01-15T14:30:05Z"
}
```

#### PUT /orders/{id}

Updates an order.

**Request Body:**
```json
{
  "quantity": 150,
  "price": 185.00
}
```

#### DELETE /orders/{id}

Cancels an order.

### Strategies

#### GET /strategies

Retrieves all strategies.

**Response:**
```json
{
  "strategies": [
    {
      "id": "strategy_123",
      "name": "EMA Cross Strategy",
      "description": "Simple moving average crossover strategy",
      "code": "# Strategy code here",
      "parameters": {
        "fast_period": 12,
        "slow_period": 26,
        "quantity": 100
      },
      "symbols": ["AAPL", "GOOGL"],
      "is_active": true,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

#### POST /strategies

Creates a new strategy.

**Request Body:**
```json
{
  "name": "RSI Strategy",
  "description": "RSI-based mean reversion strategy",
  "code": "# Strategy code here",
  "parameters": {
    "rsi_period": 14,
    "oversold": 30,
    "overbought": 70,
    "quantity": 100
  },
  "symbols": ["MSFT"]
}
```

#### GET /strategies/{id}

Retrieves a specific strategy.

#### PUT /strategies/{id}

Updates a strategy.

#### DELETE /strategies/{id}

Deletes a strategy.

### Backtests

#### GET /backtests

Retrieves all backtests.

**Response:**
```json
{
  "backtests": [
    {
      "id": "backtest_123",
      "strategy_id": "strategy_123",
      "symbol": "AAPL",
      "start_date": "2024-01-01T00:00:00Z",
      "end_date": "2024-01-15T00:00:00Z",
      "initial_balance": 100000,
      "final_balance": 112500,
      "total_return": 0.125,
      "max_drawdown": 0.082,
      "sharpe_ratio": 1.85,
      "win_rate": 0.652,
      "status": "completed",
      "created_at": "2024-01-15T15:00:00Z"
    }
  ]
}
```

#### POST /backtests

Creates a new backtest.

**Request Body:**
```json
{
  "strategy_id": "strategy_123",
  "symbol": "AAPL",
  "start_date": "2024-01-01T00:00:00Z",
  "end_date": "2024-01-15T00:00:00Z",
  "initial_balance": 100000,
  "parameters": {
    "fast_period": 12,
    "slow_period": 26,
    "quantity": 100
  }
}
```

#### GET /backtests/{id}

Retrieves a specific backtest with detailed results.

**Response:**
```json
{
  "id": "backtest_123",
  "strategy_id": "strategy_123",
  "symbol": "AAPL",
  "start_date": "2024-01-01T00:00:00Z",
  "end_date": "2024-01-15T00:00:00Z",
  "initial_balance": 100000,
  "final_balance": 112500,
  "performance": {
    "total_return": 0.125,
    "annualized_return": 0.365,
    "sharpe_ratio": 1.85,
    "max_drawdown": 0.082,
    "win_rate": 0.652,
    "profit_factor": 1.45,
    "sqn": 2.1,
    "cagr": 0.365
  },
  "trades": [
    {
      "id": "trade_123",
      "symbol": "AAPL",
      "side": "BUY",
      "quantity": 100,
      "entry_price": 180.00,
      "exit_price": 185.50,
      "entry_time": "2024-01-10T10:00:00Z",
      "exit_time": "2024-01-12T14:30:00Z",
      "pnl": 550.00,
      "commission": 2.00
    }
  ],
  "equity": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "equity": 100000,
      "drawdown": 0
    }
  ],
  "status": "completed",
  "created_at": "2024-01-15T15:00:00Z"
}
```

#### DELETE /backtests/{id}

Deletes a backtest.

### Universe

#### GET /universe

Retrieves all symbols in the universe.

**Response:**
```json
{
  "symbols": [
    {
      "id": 1,
      "symbol": "AAPL",
      "exchange": "NASDAQ",
      "name": "Apple Inc.",
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST /universe

Adds a symbol to the universe.

**Request Body:**
```json
{
  "symbol": "TSLA",
  "exchange": "NASDAQ",
  "name": "Tesla, Inc."
}
```

#### DELETE /universe/{symbol}

Removes a symbol from the universe.

## Error Responses

All endpoints may return the following error responses:

### 400 Bad Request
```json
{
  "error": "validation_error",
  "message": "Invalid request parameters",
  "details": {
    "field": "symbol",
    "issue": "Symbol is required"
  }
}
```

### 401 Unauthorized
```json
{
  "error": "unauthorized",
  "message": "Authentication required"
}
```

### 404 Not Found
```json
{
  "error": "not_found",
  "message": "Resource not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "internal_error",
  "message": "An internal error occurred"
}
```

## Rate Limiting

The API implements rate limiting to prevent abuse. Limits are:

- 100 requests per minute for authenticated users
- 10 requests per minute for unauthenticated users

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1703123456
```

## WebSocket Events

The API also provides WebSocket endpoints for real-time data:

### /ws/market-data

Real-time market data stream.

**Message Format:**
```json
{
  "type": "market_data",
  "symbol": "AAPL",
  "price": 185.50,
  "volume": 1000000,
  "timestamp": "2024-01-15T14:30:00Z"
}
```

### /ws/trades

Real-time trade execution stream.

**Message Format:**
```json
{
  "type": "trade",
  "order_id": "order_123",
  "trade_id": "trade_123",
  "symbol": "AAPL",
  "side": "BUY",
  "quantity": 100,
  "price": 185.50,
  "timestamp": "2024-01-15T14:30:00Z"
}
```

## SDKs and Libraries

### Go Client
```go
import "github.com/moomoo-trading/api-client-go"

client := api.NewClient("http://localhost:8080")
orders, err := client.GetOrders()
```

### JavaScript Client
```javascript
import { MoomooClient } from '@moomoo-trading/api-client-js';

const client = new MoomooClient('http://localhost:8080');
const orders = await client.getOrders();
```

## Support

For API support and questions, please contact:

- Email: api-support@moomoo-trading.com
- Documentation: https://docs.moomoo-trading.com
- GitHub Issues: https://github.com/moomoo-trading/api/issues