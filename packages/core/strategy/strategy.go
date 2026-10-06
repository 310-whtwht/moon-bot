// Package strategy defines trading strategies. A strategy only decides; risk
// checks and order execution happen elsewhere, so the same code runs in
// backtests, paper trading and live trading.
package strategy

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Bar is the closed candle a strategy sees (BID prices).
type Bar struct {
	Time  time.Time
	Open  float64
	High  float64
	Low   float64
	Close float64
}

// Side of a position.
type Side string

const (
	Long  Side = "LONG"
	Short Side = "SHORT"
)

// Position is the strategy's current open position, if any.
type Position struct {
	Side       Side
	EntryPrice float64
	EntryTime  time.Time
}

// Action is what the strategy wants to do at the next bar open.
type Action string

const (
	Hold       Action = "HOLD"
	EnterLong  Action = "ENTER_LONG"  // closes a short first, if any
	EnterShort Action = "ENTER_SHORT" // closes a long first, if any
	Exit       Action = "EXIT"
)

// Signal is a strategy decision made on a closed bar.
type Signal struct {
	Action Action
	// StopLoss is the protective stop price for a new position (required for entries).
	StopLoss float64
	Reason   string
}

// Strategy is stateful: it is fed every closed bar in order. Create a fresh
// instance per run with Definition.New.
type Strategy interface {
	OnBar(bar Bar, pos *Position) Signal
}

// Explainer is optionally implemented by strategies that can describe what
// they saw on the last bar (indicator values), for logs.
type Explainer interface {
	Explain() string
}

// ParamSpec describes one numeric parameter, used for validation and UI forms.
type ParamSpec struct {
	Name        string  `json:"name"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Integer     bool    `json:"integer"`
	Default     float64 `json:"default"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
}

// Params are numeric strategy parameters by name.
type Params map[string]float64

// Definition is a registered strategy type.
type Definition struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Params      []ParamSpec `json:"params"`
	// Scripted types take their rules, and their parameters, from a script.
	Scripted bool `json:"scripted"`
	// Validate checks cross-parameter rules after defaults are applied (optional).
	Validate func(Params) error    `json:"-"`
	Factory  func(Params) Strategy `json:"-"`
}

// Resolve applies defaults and validates params against the specs.
func (d Definition) Resolve(in Params) (Params, error) {
	known := make(map[string]ParamSpec, len(d.Params))
	out := make(Params, len(d.Params))
	for _, p := range d.Params {
		known[p.Name] = p
		out[p.Name] = p.Default
	}
	for name, v := range in {
		spec, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("%s: unknown parameter %q", d.Type, name)
		}
		if math.IsNaN(v) || v < spec.Min || v > spec.Max {
			return nil, fmt.Errorf("%s: %s=%v out of range [%v, %v]", d.Type, name, v, spec.Min, spec.Max)
		}
		if spec.Integer && v != math.Trunc(v) {
			return nil, fmt.Errorf("%s: %s must be an integer", d.Type, name)
		}
		out[name] = v
	}
	if d.Validate != nil {
		if err := d.Validate(out); err != nil {
			return nil, fmt.Errorf("%s: %w", d.Type, err)
		}
	}
	return out, nil
}

// New resolves params and creates a fresh strategy instance.
func (d Definition) New(in Params) (Strategy, Params, error) {
	params, err := d.Resolve(in)
	if err != nil {
		return nil, nil, err
	}
	return d.Factory(params), params, nil
}

var registry = map[string]Definition{}

// Register adds a strategy type. Called from init in each strategy file.
func Register(d Definition) {
	if _, dup := registry[d.Type]; dup {
		panic("strategy: duplicate type " + d.Type)
	}
	registry[d.Type] = d
}

// Lookup returns a registered strategy type.
func Lookup(typ string) (Definition, error) {
	d, ok := registry[typ]
	if !ok {
		return Definition{}, fmt.Errorf("unknown strategy type %q", typ)
	}
	return d, nil
}

// Definitions returns all registered types sorted by Type.
func Definitions() []Definition {
	out := make([]Definition, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
