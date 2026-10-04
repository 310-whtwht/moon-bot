module github.com/moomoo-trading/bot

go 1.24.2

replace github.com/moomoo-trading/core => ../../packages/core

require (
	github.com/go-sql-driver/mysql v1.10.1
	github.com/google/uuid v1.6.0
	github.com/moomoo-trading/core v0.0.0
	github.com/redis/go-redis/v9 v9.22.0
	github.com/shopspring/decimal v1.4.0
	github.com/stretchr/testify v1.12.1
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
