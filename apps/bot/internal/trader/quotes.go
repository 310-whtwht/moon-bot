package trader

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
)

// Symbols returns the distinct symbols of all deployments, sorted.
func Symbols(ctx context.Context, store Store) ([]string, error) {
	deployments, err := store.Deployments(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range deployments {
		if !seen[d.Symbol] {
			seen[d.Symbol] = true
			out = append(out, d.Symbol)
		}
	}
	sort.Strings(out)
	return out, nil
}

// StreamQuotes subscribes to ticks for the deployments' symbols and
// re-subscribes when the set of symbols changes (checked every interval), so
// a deployment added later also gets its stop-loss watched.
func StreamQuotes(ctx context.Context, source broker.MarketData, store Store, interval time.Duration, logf func(string, ...any)) <-chan market.Tick {
	out := make(chan market.Tick, 256)

	go func() {
		defer close(out)
		var (
			current string
			cancel  context.CancelFunc = func() {}
			ticks   <-chan market.Tick
		)
		defer func() { cancel() }()

		resubscribe := func() {
			symbols, err := Symbols(ctx, store)
			if err != nil {
				logf("quotes: load symbols: %v", err)
				return
			}
			key := strings.Join(symbols, ",")
			if key == current {
				return
			}
			cancel()
			current, ticks = key, nil
			if len(symbols) == 0 {
				cancel = func() {}
				return
			}
			subCtx, c := context.WithCancel(ctx)
			cancel = c
			ch, err := source.SubscribeTicks(subCtx, symbols)
			if err != nil {
				logf("quotes: subscribe %s: %v", key, err)
				current = "" // retry on the next check
				return
			}
			ticks = ch
			logf("quotes: watching %s", key)
		}

		resubscribe()
		check := time.NewTicker(interval)
		defer check.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-check.C:
				resubscribe()
			case tick, ok := <-ticks:
				if !ok {
					ticks, current = nil, "" // stream ended; resubscribe on the next check
					continue
				}
				select {
				case out <- tick:
				default: // drop when the consumer is behind; a newer quote follows
				}
			}
		}
	}()

	return out
}
