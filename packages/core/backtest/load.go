package backtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
)

// yen is the account currency: balances, profit and loss are all in it.
const yen = "JPY"

// QuoteCurrency returns the currency a symbol's prices are in ("USD" for EUR_USD).
func QuoteCurrency(symbol string) string {
	if i := strings.LastIndex(symbol, "_"); i >= 0 {
		return symbol[i+1:]
	}
	return ""
}

// ConversionSymbol returns the pair that turns a symbol's quote currency into
// yen ("USD_JPY" for EUR_USD), or "" when the symbol is already quoted in yen.
func ConversionSymbol(symbol string) string {
	quote := QuoteCurrency(symbol)
	if quote == yen || quote == "" {
		return ""
	}
	return quote + "_" + yen
}

// LoadCandles reads BID and ASK bars from the store and pairs them by open
// time. Periods missing either side are dropped.
//
// For a symbol not quoted in yen, each candle also carries the yen value of
// one unit of its quote currency, read from the conversion pair's stored bars
// of the same timeframe (the latest one at or before the candle).
func LoadCandles(ctx context.Context, store marketdata.BarStore, key market.InstrumentKey, tf market.Timeframe, from, to time.Time) ([]Candle, error) {
	bids, err := store.Bars(ctx, key, tf, market.PriceBid, from, to)
	if err != nil {
		return nil, err
	}
	asks, err := store.Bars(ctx, key, tf, market.PriceAsk, from, to)
	if err != nil {
		return nil, err
	}

	askAt := make(map[time.Time]market.Bar, len(asks))
	for _, a := range asks {
		askAt[a.OpenTime] = a
	}
	candles := make([]Candle, 0, len(bids))
	for _, b := range bids {
		a, ok := askAt[b.OpenTime]
		if !ok {
			continue
		}
		candles = append(candles, Candle{
			Time: b.OpenTime,
			Bid:  OHLC{b.Open.InexactFloat64(), b.High.InexactFloat64(), b.Low.InexactFloat64(), b.Close.InexactFloat64()},
			Ask:  OHLC{a.Open.InexactFloat64(), a.High.InexactFloat64(), a.Low.InexactFloat64(), a.Close.InexactFloat64()},
		})
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("backtest: no paired BID/ASK bars for %s %s in %s..%s (run backfill first)",
			key, tf, from.Format("2006-01-02"), to.Format("2006-01-02"))
	}

	conversion := ConversionSymbol(key.Symbol)
	if conversion == "" {
		return candles, nil
	}
	// A little earlier than the range, so the first candles have a rate too
	// (the two pairs can have a bar missing at different times).
	rates, err := store.Bars(ctx, market.InstrumentKey{Broker: key.Broker, Symbol: conversion}, tf, market.PriceBid, from.AddDate(0, 0, -7), to)
	if err != nil {
		return nil, err
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("backtest: %s is quoted in %s: %s %s bars are needed to convert profit and loss to yen (run backfill for %s first)",
			key.Symbol, QuoteCurrency(key.Symbol), conversion, tf, conversion)
	}
	next := 0
	rate := rates[0].Open.InexactFloat64() // for candles older than every rate bar
	for i := range candles {
		for next < len(rates) && !rates[next].OpenTime.After(candles[i].Time) {
			rate = rates[next].Close.InexactFloat64()
			next++
		}
		candles[i].QuoteRate = rate
	}
	return candles, nil
}
