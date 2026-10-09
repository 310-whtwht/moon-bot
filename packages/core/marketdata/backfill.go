package marketdata

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
)

// BackfillRequest describes which history to download.
type BackfillRequest struct {
	Symbol     string
	Timeframe  market.Timeframe
	PriceTypes []market.PriceType
	From       time.Time
	To         time.Time
}

// BackfillResult summarises one run per price type.
type BackfillResult struct {
	PriceType market.PriceType
	ResumedAt time.Time
	Stored    int
}

// Backfiller downloads bars from a broker into a store. It resumes from the
// newest stored bar, so it can be interrupted and re-run safely.
type Backfiller struct {
	Source broker.MarketData
	Store  BarStore
	Logf   func(format string, args ...any)
	// OnProgress, when set, is called after each stored chunk with the price
	// type, how far the download has got and the bars stored for that type.
	OnProgress func(pt market.PriceType, upTo time.Time, stored int)
}

func (b *Backfiller) logf(format string, args ...any) {
	if b.Logf != nil {
		b.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// chunkSize keeps each Bars call to a handful of API requests.
func chunkSize(tf market.Timeframe) time.Duration {
	switch tf {
	case market.TF4Hour, market.TF8Hour, market.TF12Hour, market.TF1Day:
		return 365 * 24 * time.Hour
	}
	return 7 * 24 * time.Hour
}

func (b *Backfiller) Run(ctx context.Context, req BackfillRequest) ([]BackfillResult, error) {
	barLen, err := req.Timeframe.Duration()
	if err != nil {
		return nil, err
	}
	if !req.From.Before(req.To) {
		return nil, fmt.Errorf("backfill: from %s must be before to %s", req.From, req.To)
	}

	key := market.InstrumentKey{Broker: b.Source.Name(), Symbol: req.Symbol}
	results := make([]BackfillResult, 0, len(req.PriceTypes))

	for _, pt := range req.PriceTypes {
		start := req.From.UTC()
		latest, ok, err := b.Store.LatestOpenTime(ctx, key, req.Timeframe, pt)
		if err != nil {
			return results, err
		}
		if ok && !latest.Add(barLen).Before(start) {
			start = latest.Add(barLen)
		}

		result := BackfillResult{PriceType: pt, ResumedAt: start}
		for chunkStart := start; chunkStart.Before(req.To); chunkStart = chunkStart.Add(chunkSize(req.Timeframe)) {
			chunkEnd := chunkStart.Add(chunkSize(req.Timeframe))
			if chunkEnd.After(req.To) {
				chunkEnd = req.To
			}

			bars, err := b.Source.Bars(ctx, broker.BarsRequest{
				Symbol: req.Symbol, Timeframe: req.Timeframe, PriceType: pt,
				From: chunkStart, To: chunkEnd,
			})
			if errors.Is(err, broker.ErrNoData) {
				continue
			}
			if err != nil {
				return append(results, result), fmt.Errorf("backfill %s %s %s..%s: %w",
					key, pt, chunkStart.Format(time.RFC3339), chunkEnd.Format(time.RFC3339), err)
			}
			if err := b.Store.UpsertBars(ctx, bars); err != nil {
				return append(results, result), err
			}
			result.Stored += len(bars)
			b.logf("backfill %s %s %s: %s..%s +%d bars (total %d)", key, req.Timeframe, pt,
				chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"), len(bars), result.Stored)
			if b.OnProgress != nil {
				b.OnProgress(pt, chunkEnd, result.Stored)
			}
		}
		results = append(results, result)
	}
	return results, nil
}
