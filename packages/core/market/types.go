// Package market defines broker-agnostic market data types shared by the
// API server, the bot and the backtester.
package market

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// InstrumentKey identifies an instrument on a specific broker, e.g. gmo:USD_JPY.
type InstrumentKey struct {
	Broker string
	Symbol string
}

func (k InstrumentKey) String() string {
	return k.Broker + ":" + k.Symbol
}

// ParseInstrumentKey parses the "broker:symbol" form.
func ParseInstrumentKey(s string) (InstrumentKey, error) {
	broker, symbol, ok := strings.Cut(s, ":")
	if !ok || broker == "" || symbol == "" {
		return InstrumentKey{}, fmt.Errorf("invalid instrument key %q (want broker:symbol)", s)
	}
	return InstrumentKey{Broker: broker, Symbol: symbol}, nil
}

// AssetClass is the kind of product an instrument represents.
type AssetClass string

const (
	AssetForex AssetClass = "forex"
	AssetStock AssetClass = "stock"
)

// Instrument holds the trading rules a broker reports for a symbol.
// Values must be read from the broker, never hard-coded.
type Instrument struct {
	Key           InstrumentKey
	AssetClass    AssetClass
	QuoteCurrency string
	MinOrderSize  decimal.Decimal
	MaxOrderSize  decimal.Decimal
	SizeStep      decimal.Decimal
	TickSize      decimal.Decimal
}

// MarketStatus is the trading state of a broker or instrument.
type MarketStatus string

const (
	StatusOpen        MarketStatus = "OPEN"
	StatusClosed      MarketStatus = "CLOSE"
	StatusMaintenance MarketStatus = "MAINTENANCE"
)

// PriceType selects which side of the quote a bar is built from.
type PriceType string

const (
	PriceBid PriceType = "BID"
	PriceAsk PriceType = "ASK"
)

// Timeframe is a bar interval.
type Timeframe string

const (
	TF1Min   Timeframe = "1m"
	TF5Min   Timeframe = "5m"
	TF15Min  Timeframe = "15m"
	TF30Min  Timeframe = "30m"
	TF1Hour  Timeframe = "1h"
	TF4Hour  Timeframe = "4h"
	TF8Hour  Timeframe = "8h"
	TF12Hour Timeframe = "12h"
	TF1Day   Timeframe = "1d"
)

var timeframeDurations = map[Timeframe]time.Duration{
	TF1Min:   time.Minute,
	TF5Min:   5 * time.Minute,
	TF15Min:  15 * time.Minute,
	TF30Min:  30 * time.Minute,
	TF1Hour:  time.Hour,
	TF4Hour:  4 * time.Hour,
	TF8Hour:  8 * time.Hour,
	TF12Hour: 12 * time.Hour,
	TF1Day:   24 * time.Hour,
}

// Duration returns the length of one bar.
func (tf Timeframe) Duration() (time.Duration, error) {
	d, ok := timeframeDurations[tf]
	if !ok {
		return 0, fmt.Errorf("unsupported timeframe %q", tf)
	}
	return d, nil
}

// Tick is the latest quote for an instrument.
type Tick struct {
	Key    InstrumentKey
	Bid    decimal.Decimal
	Ask    decimal.Decimal
	Time   time.Time
	Status MarketStatus
}

// Bar is one OHLC candle. OpenTime is always UTC.
type Bar struct {
	Key       InstrumentKey
	Timeframe Timeframe
	PriceType PriceType
	OpenTime  time.Time
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
}
