module github.com/moomoo-trading/bot

go 1.24.2

replace github.com/moomoo-trading/core => ../../packages/core

require (
	github.com/go-sql-driver/mysql v1.10.1
	github.com/moomoo-trading/core v0.0.0
	github.com/stretchr/testify v1.12.1
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)
