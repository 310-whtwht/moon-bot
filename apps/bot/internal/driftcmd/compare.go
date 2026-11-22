package driftcmd

import (
	"fmt"
	"io"
	"math"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/strategy"
)

// Trade is a closed position as the bot recorded it.
type Trade struct {
	Long          bool
	Units         float64
	OpenPrice     float64
	ExpectedOpen  float64 // 0 when not recorded
	ClosePrice    float64
	ExpectedClose float64 // 0 when not recorded
	PnL           float64 // net of fees, JPY
	ExitReason    string
	VersionID     string
	OpenedAt      time.Time
	ClosedAt      time.Time
}

// Pair is a bot trade and the backtest trade it corresponds to.
type Pair struct {
	Live Trade
	BT   backtest.Trade
	// EntryCost / ExitCost are how much worse the bot's fill was than the
	// backtest's, in price (positive = worse for the bot).
	EntryCost, ExitCost float64
	// BTPnL is the backtest's P&L scaled to the bot's position size.
	BTPnL float64
}

// Report is the comparison for one period.
type Report struct {
	Pairs    []Pair
	LiveOnly []Trade          // the bot traded, the backtest did not
	BTOnly   []backtest.Trade // the backtest traded, the bot did not
	// Slippage is fill minus the quote each order was sent at, worse = positive,
	// over the orders where the quote was recorded.
	EntrySlip, ExitSlip []float64
}

// Compare pairs each bot trade with the backtest trade on the same side whose
// entry bar is the one the bot entered on (within one bar: the bot fills a
// few seconds after the bar opens, the backtest at the bar's open).
func Compare(live []Trade, bt []backtest.Trade, bar time.Duration, btUnits float64) Report {
	var r Report
	used := make([]bool, len(bt))
	for _, t := range live {
		best := -1
		var bestGap time.Duration
		for i, b := range bt {
			if used[i] || (b.Side == strategy.Long) != t.Long {
				continue
			}
			gap := t.OpenedAt.Sub(b.EntryTime)
			if gap < 0 {
				gap = -gap
			}
			if gap < bar && (best < 0 || gap < bestGap) {
				best, bestGap = i, gap
			}
		}
		if t.ExpectedOpen > 0 {
			r.EntrySlip = append(r.EntrySlip, worse(t.Long, t.OpenPrice, t.ExpectedOpen))
		}
		if t.ExpectedClose > 0 {
			r.ExitSlip = append(r.ExitSlip, worse(!t.Long, t.ClosePrice, t.ExpectedClose))
		}
		if best < 0 {
			r.LiveOnly = append(r.LiveOnly, t)
			continue
		}
		used[best] = true
		b := bt[best]
		scale := 1.0
		if btUnits > 0 {
			scale = t.Units / btUnits
		}
		r.Pairs = append(r.Pairs, Pair{
			Live: t, BT: b,
			EntryCost: worse(t.Long, t.OpenPrice, b.EntryPrice),
			ExitCost:  worse(!t.Long, t.ClosePrice, b.ExitPrice),
			BTPnL:     b.PnL * scale,
		})
	}
	for i, b := range bt {
		if !used[i] {
			r.BTOnly = append(r.BTOnly, b)
		}
	}
	return r
}

// worse is how much worse price is than ref for a buy (buying higher) or a
// sell (selling lower).
func worse(buy bool, price, ref float64) float64 {
	if buy {
		return price - ref
	}
	return ref - price
}

// Add merges another report into r.
func (r *Report) Add(o Report) {
	r.Pairs = append(r.Pairs, o.Pairs...)
	r.LiveOnly = append(r.LiveOnly, o.LiveOnly...)
	r.BTOnly = append(r.BTOnly, o.BTOnly...)
	r.EntrySlip = append(r.EntrySlip, o.EntrySlip...)
	r.ExitSlip = append(r.ExitSlip, o.ExitSlip...)
}

// MatchRate is the share of all trades (either side) that were matched.
func (r Report) MatchRate() float64 {
	total := len(r.Pairs) + len(r.LiveOnly) + len(r.BTOnly)
	if total == 0 {
		return 0
	}
	return float64(len(r.Pairs)) / float64(total)
}

// Print writes the report. With detail, every trade is listed.
func (r Report) Print(out io.Writer, detail bool) {
	if detail {
		// One line per trade, in time order, whichever side took it.
		type line struct {
			at   time.Time
			text string
		}
		var lines []line
		for _, p := range r.Pairs {
			lines = append(lines, line{p.Live.OpenedAt, fmt.Sprintf("%s\t%s\t%.3f / %.3f\t%.3f / %.3f\t%.0f / %.0f\t%s / %s\t",
				p.Live.OpenedAt.Format("2006-01-02 15:04"), side(p.Live.Long),
				p.Live.OpenPrice, p.BT.EntryPrice, p.Live.ClosePrice, p.BT.ExitPrice,
				p.Live.PnL, p.BTPnL, orDash(p.Live.ExitReason), p.BT.ExitReason)})
		}
		for _, t := range r.LiveOnly {
			lines = append(lines, line{t.OpenedAt, fmt.Sprintf("%s\t%s\t%.3f / -\t%.3f / -\t%.0f / -\t%s / (backtest did not trade)\t",
				t.OpenedAt.Format("2006-01-02 15:04"), side(t.Long), t.OpenPrice, t.ClosePrice, t.PnL, orDash(t.ExitReason))})
		}
		for _, b := range r.BTOnly {
			lines = append(lines, line{b.EntryTime, fmt.Sprintf("%s\t%s\t- / %.3f\t- / %.3f\t- / %.0f\t(bot did not trade) / %s\t",
				b.EntryTime.Format("2006-01-02 15:04"), side(b.Side == strategy.Long), b.EntryPrice, b.ExitPrice, b.PnL, b.ExitReason)})
		}
		sort.SliceStable(lines, func(i, j int) bool { return lines[i].at.Before(lines[j].at) })
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "opened (UTC)\tside\tentry bot/bt\texit bot/bt\tP&L bot/bt\texit bot/bt\t")
		for _, l := range lines {
			fmt.Fprintln(w, l.text)
		}
		w.Flush()
		fmt.Fprintln(out)
	}

	var livePnL, btPnL float64
	var entry, exit []float64
	for _, p := range r.Pairs {
		livePnL += p.Live.PnL
		btPnL += p.BTPnL
		entry = append(entry, p.EntryCost)
		exit = append(exit, p.ExitCost)
	}
	fmt.Fprintf(out, "matched %d, bot only %d, backtest only %d (match rate %.0f%%)\n",
		len(r.Pairs), len(r.LiveOnly), len(r.BTOnly), r.MatchRate()*100)
	if len(r.Pairs) > 0 {
		fmt.Fprintf(out, "matched P&L: bot %.0f JPY, backtest %.0f JPY (difference %.0f JPY, %.1f JPY per trade)\n",
			livePnL, btPnL, livePnL-btPnL, (livePnL-btPnL)/float64(len(r.Pairs)))
		fmt.Fprintf(out, "fill vs backtest (worse = +): entry median %s, exit median %s\n", price(median(entry)), price(median(exit)))
	}
	if len(r.EntrySlip)+len(r.ExitSlip) > 0 {
		fmt.Fprintf(out, "fill vs quote sent at (worse = +): entry median %s (%d), exit median %s (%d)\n",
			price(median(r.EntrySlip)), len(r.EntrySlip), price(median(r.ExitSlip)), len(r.ExitSlip))
	}
}

func side(long bool) string {
	if long {
		return "buy"
	}
	return "sell"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func price(v float64) string {
	if math.IsNaN(v) {
		return "-"
	}
	return fmt.Sprintf("%+.4f", v)
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}
