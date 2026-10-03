// Package indicator implements streaming technical indicators. Each indicator
// is fed one closed bar at a time, so a strategy costs O(1) per bar in both
// backtests and live trading.
package indicator

import "math"

// EMA is an exponential moving average seeded with the simple average of the
// first Period values (the common charting convention).
type EMA struct {
	period int
	k      float64
	count  int
	sum    float64
	value  float64
}

func NewEMA(period int) *EMA {
	return &EMA{period: period, k: 2 / float64(period+1)}
}

// Update adds a value and returns the current EMA and whether it is ready.
func (e *EMA) Update(v float64) (float64, bool) {
	e.count++
	if e.count <= e.period {
		e.sum += v
		if e.count < e.period {
			return 0, false
		}
		e.value = e.sum / float64(e.period)
		return e.value, true
	}
	e.value += e.k * (v - e.value)
	return e.value, true
}

func (e *EMA) Value() (float64, bool) {
	return e.value, e.count >= e.period
}

// ATR is the average true range with Wilder's smoothing, seeded with the
// simple average of the first Period true ranges.
type ATR struct {
	period    int
	count     int
	sum       float64
	value     float64
	prevClose float64
	hasPrev   bool
}

func NewATR(period int) *ATR {
	return &ATR{period: period}
}

// Update adds a bar and returns the current ATR and whether it is ready.
func (a *ATR) Update(high, low, close float64) (float64, bool) {
	tr := high - low
	if a.hasPrev {
		tr = math.Max(tr, math.Max(math.Abs(high-a.prevClose), math.Abs(low-a.prevClose)))
	}
	a.prevClose, a.hasPrev = close, true

	a.count++
	if a.count <= a.period {
		a.sum += tr
		if a.count < a.period {
			return 0, false
		}
		a.value = a.sum / float64(a.period)
		return a.value, true
	}
	a.value = (a.value*float64(a.period-1) + tr) / float64(a.period)
	return a.value, true
}

func (a *ATR) Value() (float64, bool) {
	return a.value, a.count >= a.period
}
