package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	d, err := Lookup("ema_cross")
	require.NoError(t, err)

	p, err := d.Resolve(Params{"fast_period": 5})
	require.NoError(t, err)
	assert.Equal(t, 5.0, p["fast_period"])
	assert.Equal(t, 26.0, p["slow_period"]) // default applied

	_, err = d.Resolve(Params{"unknown": 1})
	assert.Error(t, err)
	_, err = d.Resolve(Params{"fast_period": 1}) // below min
	assert.Error(t, err)
	_, err = d.Resolve(Params{"fast_period": 5.5}) // not an integer
	assert.Error(t, err)
	_, err = d.Resolve(Params{"fast_period": 30, "slow_period": 20}) // cross-param rule
	assert.Error(t, err)

	_, err = Lookup("nope")
	assert.Error(t, err)
	assert.NotEmpty(t, Definitions())
}

// feed runs closes through a strategy, tracking the position it asks for.
func feed(s Strategy, closes []float64) []Signal {
	var pos *Position
	var signals []Signal
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		bar := Bar{Time: start.Add(time.Duration(i) * time.Hour), Open: c, High: c + 0.1, Low: c - 0.1, Close: c}
		sig := s.OnBar(bar, pos)
		signals = append(signals, sig)
		switch sig.Action {
		case EnterLong:
			pos = &Position{Side: Long, EntryPrice: c}
		case EnterShort:
			pos = &Position{Side: Short, EntryPrice: c}
		case Exit:
			pos = nil
		}
	}
	return signals
}

func actions(signals []Signal) []Action {
	var out []Action
	for _, s := range signals {
		if s.Action != Hold {
			out = append(out, s.Action)
		}
	}
	return out
}

// downUpDown is a series that falls, rises, then falls again.
func downUpDown() []float64 {
	var closes []float64
	for i := 0; i < 15; i++ {
		closes = append(closes, 100-float64(i))
	}
	for i := 0; i < 15; i++ {
		closes = append(closes, 86+float64(i)*2)
	}
	for i := 0; i < 15; i++ {
		closes = append(closes, 114-float64(i)*2)
	}
	return closes
}

func TestEMACross_LongAndShort(t *testing.T) {
	d, _ := Lookup("ema_cross")
	s, _, err := d.New(Params{"fast_period": 3, "slow_period": 6, "atr_period": 3, "stop_atr_mult": 2})
	require.NoError(t, err)

	signals := feed(s, downUpDown())
	assert.Equal(t, []Action{EnterLong, EnterShort}, actions(signals))

	for i, sig := range signals {
		bar := downUpDown()[i]
		switch sig.Action {
		case EnterLong:
			assert.Less(t, sig.StopLoss, bar, "long stop below close")
		case EnterShort:
			assert.Greater(t, sig.StopLoss, bar, "short stop above close")
		}
	}
}

func TestEMACross_LongOnlyExits(t *testing.T) {
	d, _ := Lookup("ema_cross")
	s, _, err := d.New(Params{"fast_period": 3, "slow_period": 6, "atr_period": 3, "allow_short": 0})
	require.NoError(t, err)

	assert.Equal(t, []Action{EnterLong, Exit}, actions(feed(s, downUpDown())))
}

func TestEMACross_HoldsDuringWarmup(t *testing.T) {
	d, _ := Lookup("ema_cross")
	s, _, err := d.New(nil)
	require.NoError(t, err)

	for _, sig := range feed(s, []float64{1, 2, 3, 4, 5}) {
		assert.Equal(t, Hold, sig.Action)
	}
}

func TestEMACross_ExplainsWhatItSaw(t *testing.T) {
	d, _ := Lookup("ema_cross")
	s, _, err := d.New(Params{"fast_period": 2, "slow_period": 3, "atr_period": 2})
	require.NoError(t, err)
	e, ok := s.(Explainer)
	require.True(t, ok)

	feed(s, []float64{100, 100})
	assert.Equal(t, "warming up", e.Explain())

	feed(s, []float64{100, 103})
	// fast: 100 -> 102, slow: 100 -> 101.5
	assert.Contains(t, e.Explain(), "fast=102.000 slow=101.500 diff=+0.500")
}
