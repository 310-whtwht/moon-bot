package backtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
)

// LoadCandles reads BID and ASK bars from the store and pairs them by open
// time. Periods missing either side are dropped.
func LoadCandles(ctx context.Context, store marketdata.BarStore, key market.InstrumentKey, tf market.Timeframe, from, to time.Time) ([]Candle, error) {
	if !strings.HasSuffix(key.Symbol, "_JPY") {
		return nil, fmt.Errorf("backtest: only JPY-quoted symbols are supported (got %s)", key.Symbol)
	}
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
	return candles, nil
}
