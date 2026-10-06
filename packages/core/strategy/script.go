package strategy

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/moomoo-trading/core/indicator"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ScriptType is the strategy type whose rules are written as a script instead
// of being built into the program.
//
// A script is Starlark (a small, safe dialect of Python: no files, no
// network, no imports). It defines one function, called once per closed bar:
//
//	PARAMS = {"fast": 12, "slow": 26, "stop_atr": 2.0}   # optional, numbers
//
//	def on_bar(bar, pos):
//	    f, s = ema(p.fast), ema(p.slow)
//	    f1, s1 = ema(p.fast, ago=1), ema(p.slow, ago=1)
//	    if None in (f, s, f1, s1, atr(14)):
//	        return hold()
//	    if f1 <= s1 and f > s and (pos == None or pos.side == "short"):
//	        return buy(stop=bar.close - p.stop_atr * atr(14), reason="golden cross")
//	    return hold()
//
// See docs/strategy-scripts.md for everything a script can use.
const ScriptType = "script"

const (
	scriptFile = "strategy.star"
	// maxScriptSteps bounds one call, so a loop that never ends cannot hang the bot.
	maxScriptSteps = 5_000_000
	maxScriptBytes = 64 * 1024
)

// jst is where a bar's hour and weekday are read: the market's rhythm
// (Tokyo open, the 06:00 roll-over) is described in Japan time.
var jst = time.FixedZone("JST", 9*60*60)

// Failer is optionally implemented by strategies that can fail while running
// (a script error). After a failure OnBar keeps returning Hold; callers should
// surface Err rather than trade on.
type Failer interface {
	Err() error
}

func init() {
	Register(Definition{
		Type:        ScriptType,
		Name:        "スクリプト",
		Description: "売買ルールをスクリプト（Starlark）で書く。パラメータはスクリプトの PARAMS で定義する。",
		Scripted:    true,
		Params:      []ParamSpec{}, // known once a script is given
		Factory:     func(Params) Strategy { return failed{errors.New("script strategy has no script")} },
	})
}

// failed is a strategy that never trades and reports why.
type failed struct{ err error }

func (f failed) OnBar(Bar, *Position) Signal { return Signal{Action: Hold} }
func (f failed) Err() error                  { return f.err }
func (f failed) Explain() string             { return "script error: " + f.err.Error() }

// Define returns the definition to run for a strategy type. For built-in
// types the script is ignored; for ScriptType the script is compiled and its
// PARAMS become the definition's parameters.
func Define(typ, script string) (Definition, error) {
	if typ != ScriptType {
		return Lookup(typ)
	}
	return compileScript(script)
}

func compileScript(src string) (Definition, error) {
	if len(src) == 0 {
		return Definition{}, errors.New("script: the script is empty")
	}
	if len(src) > maxScriptBytes {
		return Definition{}, fmt.Errorf("script: longer than %d bytes", maxScriptBytes)
	}
	// Run it once to find syntax errors and read PARAMS.
	probe, err := newScriptRun(src, nil)
	if err != nil {
		return Definition{}, err
	}

	specs := make([]ParamSpec, 0, len(probe.defaults))
	for name, value := range probe.defaults {
		specs = append(specs, ParamSpec{Name: name, Label: name, Default: value, Min: -1e12, Max: 1e12})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })

	def, _ := Lookup(ScriptType)
	def.Params = specs
	def.Factory = func(p Params) Strategy {
		run, err := newScriptRun(src, p)
		if err != nil {
			return failed{err}
		}
		return run
	}
	return def, nil
}

// scriptRun is one running instance of a script.
type scriptRun struct {
	onBar    *starlark.Function
	defaults map[string]float64
	state    *starlark.Dict

	opens, highs, lows, closes []float64
	// series caches indicator values per bar (NaN until ready), extended on demand.
	series map[string]*scriptSeries

	explain string
	err     error
}

type scriptSeries struct {
	values []float64
	update func(i int) (float64, bool)
}

// newScriptRun executes the script's top level. params overrides PARAMS; nil
// is used when only compiling.
func newScriptRun(src string, params Params) (*scriptRun, error) {
	r := &scriptRun{state: starlark.NewDict(0), series: map[string]*scriptSeries{}}

	// PARAMS is read from a first pass, so that `p` can be predeclared for the real one.
	defaults, err := scriptParams(src, r)
	if err != nil {
		return nil, err
	}
	r.defaults = defaults

	values := starlark.StringDict{}
	for name, v := range defaults {
		values[name] = starlark.Float(v)
	}
	for name, v := range params {
		if _, ok := defaults[name]; ok {
			values[name] = starlark.Float(v)
		}
	}
	globals, err := r.exec(src, starlarkstruct.FromStringDict(starlark.String("params"), values))
	if err != nil {
		return nil, err
	}
	fn, ok := globals["on_bar"].(*starlark.Function)
	if !ok {
		return nil, errors.New("script: define a function `def on_bar(bar, pos):`")
	}
	if fn.NumParams() != 2 {
		return nil, fmt.Errorf("script: on_bar must take 2 arguments (bar, pos), it takes %d", fn.NumParams())
	}
	r.onBar = fn
	return r, nil
}

// scriptParams runs the top level with an empty `p` to read PARAMS.
func scriptParams(src string, r *scriptRun) (map[string]float64, error) {
	globals, err := r.exec(src, starlarkstruct.FromStringDict(starlark.String("params"), starlark.StringDict{}))
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	raw, ok := globals["PARAMS"]
	if !ok {
		return out, nil
	}
	dict, ok := raw.(*starlark.Dict)
	if !ok {
		return nil, errors.New(`script: PARAMS must be a dict such as {"period": 20}`)
	}
	for _, item := range dict.Items() {
		name, ok := starlark.AsString(item[0])
		if !ok || name == "" {
			return nil, errors.New("script: PARAMS keys must be non-empty strings")
		}
		value, ok := starlark.AsFloat(item[1])
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("script: PARAMS[%q] must be a number", name)
		}
		out[name] = value
	}
	return out, nil
}

func (r *scriptRun) exec(src string, p starlark.Value) (starlark.StringDict, error) {
	thread := &starlark.Thread{Name: "script"}
	thread.SetMaxExecutionSteps(maxScriptSteps)
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, scriptFile, src, r.predeclared(p))
	if err != nil {
		return nil, scriptError(err)
	}
	return globals, nil
}

// scriptError keeps the script's own line numbers in the message.
func scriptError(err error) error {
	var evalErr *starlark.EvalError
	if errors.As(err, &evalErr) {
		return fmt.Errorf("script: %s", evalErr.Backtrace())
	}
	return fmt.Errorf("script: %w", err)
}

func (r *scriptRun) Err() error { return r.err }

func (r *scriptRun) Explain() string {
	if r.err != nil {
		return "script error: " + r.err.Error()
	}
	return r.explain
}

func (r *scriptRun) OnBar(bar Bar, pos *Position) Signal {
	r.opens = append(r.opens, bar.Open)
	r.highs = append(r.highs, bar.High)
	r.lows = append(r.lows, bar.Low)
	r.closes = append(r.closes, bar.Close)
	if r.err != nil {
		return Signal{Action: Hold}
	}
	r.explain = ""

	t := bar.Time.In(jst)
	barValue := starlarkstruct.FromStringDict(starlark.String("bar"), starlark.StringDict{
		"time":    starlark.MakeInt64(bar.Time.Unix()),
		"open":    starlark.Float(bar.Open),
		"high":    starlark.Float(bar.High),
		"low":     starlark.Float(bar.Low),
		"close":   starlark.Float(bar.Close),
		"hour":    starlark.MakeInt(t.Hour()),
		"minute":  starlark.MakeInt(t.Minute()),
		"weekday": starlark.MakeInt((int(t.Weekday()) + 6) % 7), // 0 = Monday
	})
	var posValue starlark.Value = starlark.None
	if pos != nil {
		side := "long"
		if pos.Side == Short {
			side = "short"
		}
		posValue = starlarkstruct.FromStringDict(starlark.String("pos"), starlark.StringDict{
			"side":  starlark.String(side),
			"entry": starlark.Float(pos.EntryPrice),
		})
	}

	thread := &starlark.Thread{Name: "on_bar"}
	thread.SetMaxExecutionSteps(maxScriptSteps)
	result, err := starlark.Call(thread, r.onBar, starlark.Tuple{barValue, posValue}, nil)
	if err != nil {
		r.err = scriptError(err)
		return Signal{Action: Hold}
	}
	sig, err := toSignal(result)
	if err != nil {
		r.err = err
		return Signal{Action: Hold}
	}
	return sig
}

// scriptSignal is what buy(), sell(), exit() and hold() return.
type scriptSignal struct{ Signal }

func (scriptSignal) String() string        { return "signal" }
func (scriptSignal) Type() string          { return "signal" }
func (scriptSignal) Freeze()               {}
func (scriptSignal) Truth() starlark.Bool  { return starlark.True }
func (scriptSignal) Hash() (uint32, error) { return 0, errors.New("unhashable type: signal") }

func toSignal(v starlark.Value) (Signal, error) {
	switch v := v.(type) {
	case starlark.NoneType:
		return Signal{Action: Hold}, nil
	case scriptSignal:
		return v.Signal, nil
	}
	return Signal{}, fmt.Errorf("script: on_bar must return buy(...), sell(...), exit(), hold() or None, got %s", v.Type())
}

// predeclared is everything a script can call.
func (r *scriptRun) predeclared(p starlark.Value) starlark.StringDict {
	entry := func(name string, action Action) *starlark.Builtin {
		return starlark.NewBuiltin(name, func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var stop starlark.Value
			reason := ""
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "stop", &stop, "reason?", &reason); err != nil {
				return nil, err
			}
			price, ok := starlark.AsFloat(stop)
			if !ok || math.IsNaN(price) || price <= 0 {
				return nil, fmt.Errorf("%s: stop must be a positive price (every entry needs a stop-loss)", b.Name())
			}
			return scriptSignal{Signal{Action: action, StopLoss: price, Reason: reason}}, nil
		})
	}
	price := func(name string, values *[]float64) *starlark.Builtin {
		return starlark.NewBuiltin(name, func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			ago := 0
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "ago?", &ago); err != nil {
				return nil, err
			}
			i := len(*values) - 1 - ago
			if ago < 0 || i < 0 {
				return starlark.None, nil
			}
			return starlark.Float((*values)[i]), nil
		})
	}
	// indicator is a builtin name(period, ago=0) backed by a per-bar series.
	indicatorFn := func(name string, build func(period int) func(i int) (float64, bool)) *starlark.Builtin {
		return starlark.NewBuiltin(name, func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var periodValue starlark.Value
			ago := 0
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "period", &periodValue, "ago?", &ago); err != nil {
				return nil, err
			}
			// Periods usually come from PARAMS, which are floats.
			pf, ok := starlark.AsFloat(periodValue)
			if !ok || pf != math.Trunc(pf) || pf < 1 || pf > 5000 {
				return nil, fmt.Errorf("%s: period must be a whole number from 1 to 5000", b.Name())
			}
			period := int(pf)
			key := fmt.Sprintf("%s/%d", name, period)
			s := r.series[key]
			if s == nil {
				s = &scriptSeries{update: build(period)}
				r.series[key] = s
			}
			for i := len(s.values); i < len(r.closes); i++ {
				v, ready := s.update(i)
				if !ready {
					v = math.NaN()
				}
				s.values = append(s.values, v)
			}
			i := len(s.values) - 1 - ago
			if ago < 0 || i < 0 || math.IsNaN(s.values[i]) {
				return starlark.None, nil
			}
			return starlark.Float(s.values[i]), nil
		})
	}
	// window is an indicator computed from the last `period` bars of a price series.
	window := func(values *[]float64, pick func(a, b float64) float64) func(int) func(int) (float64, bool) {
		return func(period int) func(int) (float64, bool) {
			return func(i int) (float64, bool) {
				if i+1 < period {
					return 0, false
				}
				out := (*values)[i]
				for j := i - period + 1; j < i; j++ {
					out = pick(out, (*values)[j])
				}
				return out, true
			}
		}
	}

	return starlark.StringDict{
		"p":     p,
		"state": r.state,

		"buy":  entry("buy", EnterLong),
		"sell": entry("sell", EnterShort),
		"exit": starlark.NewBuiltin("exit", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			reason := ""
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "reason?", &reason); err != nil {
				return nil, err
			}
			return scriptSignal{Signal{Action: Exit, Reason: reason}}, nil
		}),
		"hold": starlark.NewBuiltin("hold", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
				return nil, err
			}
			return scriptSignal{Signal{Action: Hold}}, nil
		}),
		// explain(text) sets the note shown next to this bar's decision in the log and the UI.
		"explain": starlark.NewBuiltin("explain", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			text := ""
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text); err != nil {
				return nil, err
			}
			if len(text) > 200 {
				text = text[:200]
			}
			r.explain = text
			return starlark.None, nil
		}),

		// num(x, digits=3) formats a number: Starlark's % operator has no "%.3f".
		"num": starlark.NewBuiltin("num", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var value starlark.Value
			digits := 3
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &value, "digits?", &digits); err != nil {
				return nil, err
			}
			if value == starlark.None {
				return starlark.String("-"), nil
			}
			f, ok := starlark.AsFloat(value)
			if !ok || digits < 0 || digits > 10 {
				return nil, fmt.Errorf("num: x must be a number and digits 0 to 10")
			}
			return starlark.String(strconv.FormatFloat(f, 'f', digits, 64)), nil
		}),

		"open":  price("open", &r.opens),
		"high":  price("high", &r.highs),
		"low":   price("low", &r.lows),
		"close": price("close", &r.closes),
		"bars": starlark.NewBuiltin("bars", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
			return starlark.MakeInt(len(r.closes)), nil
		}),

		"ema": indicatorFn("ema", func(period int) func(int) (float64, bool) {
			ind := indicator.NewEMA(period)
			return func(i int) (float64, bool) { return ind.Update(r.closes[i]) }
		}),
		"sma": indicatorFn("sma", func(period int) func(int) (float64, bool) {
			ind := indicator.NewSMA(period)
			return func(i int) (float64, bool) { return ind.Update(r.closes[i]) }
		}),
		"rsi": indicatorFn("rsi", func(period int) func(int) (float64, bool) {
			ind := indicator.NewRSI(period)
			return func(i int) (float64, bool) { return ind.Update(r.closes[i]) }
		}),
		"atr": indicatorFn("atr", func(period int) func(int) (float64, bool) {
			ind := indicator.NewATR(period)
			return func(i int) (float64, bool) { return ind.Update(r.highs[i], r.lows[i], r.closes[i]) }
		}),
		"highest": indicatorFn("highest", window(&r.highs, math.Max)),
		"lowest":  indicatorFn("lowest", window(&r.lows, math.Min)),
	}
}
