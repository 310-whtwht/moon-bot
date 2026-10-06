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

// SMA is a simple moving average over the last Period values.
type SMA struct {
	period int
	window []float64
	next   int
	sum    float64
}

func NewSMA(period int) *SMA {
	return &SMA{period: period, window: make([]float64, 0, period)}
}

// Update adds a value and returns the current SMA and whether it is ready.
func (s *SMA) Update(v float64) (float64, bool) {
	if len(s.window) < s.period {
		s.window = append(s.window, v)
		s.sum += v
	} else {
		s.sum += v - s.window[s.next]
		s.window[s.next] = v
		s.next = (s.next + 1) % s.period
	}
	if len(s.window) < s.period {
		return 0, false
	}
	return s.sum / float64(s.period), true
}

// RSI is the relative strength index with Wilder's smoothing, seeded with the
// simple average of the first Period gains and losses.
type RSI struct {
	period   int
	count    int // changes seen
	prev     float64
	hasPrev  bool
	avgGain  float64
	avgLoss  float64
	sumGain  float64
	sumLoss  float64
	lastRSI  float64
	hasValue bool
}

func NewRSI(period int) *RSI {
	return &RSI{period: period}
}

// Update adds a close and returns the current RSI (0-100) and whether it is ready.
func (r *RSI) Update(close float64) (float64, bool) {
	if !r.hasPrev {
		r.prev, r.hasPrev = close, true
		return 0, false
	}
	change := close - r.prev
	r.prev = close
	gain, loss := math.Max(change, 0), math.Max(-change, 0)

	r.count++
	if r.count <= r.period {
		r.sumGain += gain
		r.sumLoss += loss
		if r.count < r.period {
			return 0, false
		}
		r.avgGain, r.avgLoss = r.sumGain/float64(r.period), r.sumLoss/float64(r.period)
	} else {
		n := float64(r.period)
		r.avgGain = (r.avgGain*(n-1) + gain) / n
		r.avgLoss = (r.avgLoss*(n-1) + loss) / n
	}

	switch {
	case r.avgLoss == 0 && r.avgGain == 0:
		r.lastRSI = 50
	case r.avgLoss == 0:
		r.lastRSI = 100
	default:
		r.lastRSI = 100 - 100/(1+r.avgGain/r.avgLoss)
	}
	r.hasValue = true
	return r.lastRSI, true
}
