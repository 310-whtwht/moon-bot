package strategy

import (
	"errors"
	"fmt"

	"github.com/moomoo-trading/core/indicator"
)

func init() {
	Register(Definition{
		Type:        "ema_cross",
		Name:        "EMA クロス",
		Description: "短期 EMA が長期 EMA を上抜けたら買い、下抜けたら売り。ATR の倍数で損切りを置く。反対のクロスで決済・ドテン。",
		Params: []ParamSpec{
			{Name: "fast_period", Label: "短期 EMA 期間", Integer: true, Default: 12, Min: 2, Max: 200},
			{Name: "slow_period", Label: "長期 EMA 期間", Integer: true, Default: 26, Min: 3, Max: 400},
			{Name: "atr_period", Label: "ATR 期間", Integer: true, Default: 14, Min: 2, Max: 200},
			{Name: "stop_atr_mult", Label: "損切り幅（ATR の倍数）", Default: 2, Min: 0.5, Max: 10},
			{Name: "allow_short", Label: "売りから入る（1=する / 0=しない）", Integer: true, Default: 1, Min: 0, Max: 1},
		},
		Validate: func(p Params) error {
			if p["fast_period"] >= p["slow_period"] {
				return errors.New("fast_period must be less than slow_period")
			}
			return nil
		},
		Factory: newEMACross,
	})
}

type emaCross struct {
	fast, slow  *indicator.EMA
	atr         *indicator.ATR
	stopMult    float64
	allowShort  bool
	prevDiff    float64
	hasPrevDiff bool
	// state is what the last bar looked like, for Explain.
	state string
}

func (s *emaCross) Explain() string { return s.state }

func newEMACross(p Params) Strategy {
	return &emaCross{
		fast:       indicator.NewEMA(int(p["fast_period"])),
		slow:       indicator.NewEMA(int(p["slow_period"])),
		atr:        indicator.NewATR(int(p["atr_period"])),
		stopMult:   p["stop_atr_mult"],
		allowShort: p["allow_short"] == 1,
	}
}

func (s *emaCross) OnBar(bar Bar, pos *Position) Signal {
	fast, fastOK := s.fast.Update(bar.Close)
	slow, slowOK := s.slow.Update(bar.Close)
	atr, atrOK := s.atr.Update(bar.High, bar.Low, bar.Close)
	if !fastOK || !slowOK || !atrOK {
		s.state = "warming up"
		return Signal{Action: Hold}
	}

	diff := fast - slow
	s.state = fmt.Sprintf("fast=%.3f slow=%.3f diff=%+.3f atr=%.3f", fast, slow, diff, atr)
	prev, hadPrev := s.prevDiff, s.hasPrevDiff
	s.prevDiff, s.hasPrevDiff = diff, true
	if !hadPrev {
		return Signal{Action: Hold}
	}

	crossedUp := prev <= 0 && diff > 0
	crossedDown := prev >= 0 && diff < 0
	stop := s.stopMult * atr

	switch {
	case crossedUp && (pos == nil || pos.Side == Short):
		return Signal{Action: EnterLong, StopLoss: bar.Close - stop,
			Reason: fmt.Sprintf("golden cross fast=%.3f slow=%.3f atr=%.3f", fast, slow, atr)}
	case crossedDown && pos != nil && pos.Side == Long && !s.allowShort:
		return Signal{Action: Exit, Reason: fmt.Sprintf("dead cross fast=%.3f slow=%.3f", fast, slow)}
	case crossedDown && s.allowShort && (pos == nil || pos.Side == Long):
		return Signal{Action: EnterShort, StopLoss: bar.Close + stop,
			Reason: fmt.Sprintf("dead cross fast=%.3f slow=%.3f atr=%.3f", fast, slow, atr)}
	}
	return Signal{Action: Hold}
}
