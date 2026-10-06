package strategy

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emaCrossScript is the built-in ema_cross strategy written as a script.
const emaCrossScript = `
PARAMS = {"fast_period": 12, "slow_period": 26, "atr_period": 14, "stop_atr_mult": 2.0, "allow_short": 1}

def on_bar(bar, pos):
    fast, slow = ema(p.fast_period), ema(p.slow_period)
    fast1, slow1 = ema(p.fast_period, ago=1), ema(p.slow_period, ago=1)
    a = atr(p.atr_period)
    if None in (fast, slow, fast1, slow1, a):
        return hold()
    explain("fast=" + num(fast) + " slow=" + num(slow, digits=2))
    up = fast1 - slow1 <= 0 and fast - slow > 0
    down = fast1 - slow1 >= 0 and fast - slow < 0
    stop = p.stop_atr_mult * a
    if up and (pos == None or pos.side == "short"):
        return buy(stop=bar.close - stop, reason="golden cross")
    if down and pos != None and pos.side == "long" and p.allow_short != 1:
        return exit(reason="dead cross")
    if down and p.allow_short == 1 and (pos == None or pos.side == "long"):
        return sell(stop=bar.close + stop, reason="dead cross")
    return hold()
`

// wave is a price path with several swings, so EMAs cross both ways.
func wave(n int) []Bar {
	bars := make([]Bar, 0, n)
	price := 150.0
	t := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		step := 0.04
		if (i/45)%2 == 1 {
			step = -0.05
		}
		if i%7 == 0 {
			step = -step * 1.5
		}
		open := price
		price += step
		high, low := open, price
		if low > high {
			high, low = low, high
		}
		bars = append(bars, Bar{Time: t.Add(time.Duration(i) * time.Hour), Open: open, High: high + 0.02, Low: low - 0.02, Close: price})
	}
	return bars
}

// run feeds bars, tracking the position the signals imply.
func run(s Strategy, bars []Bar) []Signal {
	var pos *Position
	out := make([]Signal, 0, len(bars))
	for _, b := range bars {
		sig := s.OnBar(b, pos)
		switch sig.Action {
		case EnterLong:
			pos = &Position{Side: Long, EntryPrice: b.Close, EntryTime: b.Time}
		case EnterShort:
			pos = &Position{Side: Short, EntryPrice: b.Close, EntryTime: b.Time}
		case Exit:
			pos = nil
		}
		out = append(out, sig)
	}
	return out
}

func TestScript_MatchesTheBuiltInStrategyItReimplements(t *testing.T) {
	bars := wave(400)
	for _, allowShort := range []float64{1, 0} {
		params := Params{"fast_period": 5, "slow_period": 13, "atr_period": 7, "stop_atr_mult": 1.5, "allow_short": allowShort}

		builtin, err := Define("ema_cross", "")
		require.NoError(t, err)
		want, _, err := builtin.New(params)
		require.NoError(t, err)

		def, err := Define(ScriptType, emaCrossScript)
		require.NoError(t, err)
		got, _, err := def.New(params)
		require.NoError(t, err)

		wantSignals, gotSignals := run(want, bars), run(got, bars)
		require.NoError(t, got.(Failer).Err())
		assert.Regexp(t, `^fast=\d+\.\d{3} slow=\d+\.\d{2}$`, got.(Explainer).Explain())
		trades := 0
		for i := range bars {
			assert.Equal(t, wantSignals[i].Action, gotSignals[i].Action, "bar %d", i)
			assert.InDelta(t, wantSignals[i].StopLoss, gotSignals[i].StopLoss, 1e-9, "bar %d", i)
			if wantSignals[i].Action != Hold {
				trades++
			}
		}
		assert.Greater(t, trades, 4, "the comparison must cover real signals")
	}
}

func TestScript_ParamsComeFromTheScript(t *testing.T) {
	def, err := Define(ScriptType, emaCrossScript)
	require.NoError(t, err)
	require.Len(t, def.Params, 5)
	assert.Equal(t, "allow_short", def.Params[0].Name, "sorted by name")
	assert.True(t, def.Scripted)

	resolved, err := def.Resolve(Params{"fast_period": 8})
	require.NoError(t, err)
	assert.Equal(t, 8.0, resolved["fast_period"])
	assert.Equal(t, 26.0, resolved["slow_period"], "defaults from PARAMS")

	_, err = def.Resolve(Params{"no_such": 1})
	assert.ErrorContains(t, err, "unknown parameter")
}

func TestScript_BarFieldsStateAndPrices(t *testing.T) {
	def, err := Define(ScriptType, `
def on_bar(bar, pos):
    state["n"] = state.get("n", 0) + 1
    # Monday 2026-10-05 09:00 JST is 00:00 UTC.
    if bars() == 1:
        if (bar.hour, bar.minute, bar.weekday) != (9, 0, 0):
            fail("wrong time: %d:%d weekday %d" % (bar.hour, bar.minute, bar.weekday))
        if close(ago=1) != None or sma(2) != None:
            fail("no history yet")
    if bars() == 3:
        if state["n"] != 3 or close(ago=2) != 100.0 or high() != 103.5 or lowest(3) != 99.5 or highest(2) != 103.5:
            fail("history is wrong")
        if sma(2) != 102.5 or sma(2, ago=1) != 101.0:
            fail("sma is wrong")
        return sell(stop=bar.close + 1, reason="third bar")
    return None
`)
	require.NoError(t, err)
	s, _, err := def.New(nil)
	require.NoError(t, err)

	t0 := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	var last Signal
	for i, c := range []float64{100, 102, 103} {
		last = s.OnBar(Bar{Time: t0.Add(time.Duration(i) * time.Hour), Open: c, High: c + 0.5, Low: c - 0.5, Close: c}, nil)
	}
	require.NoError(t, s.(Failer).Err())
	assert.Equal(t, Signal{Action: EnterShort, StopLoss: 104, Reason: "third bar"}, last)
}

func TestScript_CompileErrors(t *testing.T) {
	cases := map[string]string{
		"":                                    "empty",
		"def on_bar(bar, pos:\n    pass":      "strategy.star:1",
		"x = 1":                               "def on_bar(bar, pos)",
		"def on_bar(bar):\n    return hold()": "2 arguments",
		"PARAMS = {\"a\": \"x\"}\ndef on_bar(bar, pos):\n    return hold()": "must be a number",
		"PARAMS = [1]\ndef on_bar(bar, pos):\n    return hold()":            "must be a dict",
		"load(\"os\", \"x\")\ndef on_bar(bar, pos):\n    return hold()":     "load",
	}
	for src, want := range cases {
		_, err := Define(ScriptType, src)
		assert.ErrorContains(t, err, want, "script: %q", src)
	}
}

func TestScript_RuntimeErrorStopsTradingAndIsReported(t *testing.T) {
	cases := map[string]string{
		"def on_bar(bar, pos):\n    return buy(stop=0)":                       "stop must be a positive price",
		"def on_bar(bar, pos):\n    return buy()":                             "missing argument for stop",
		"def on_bar(bar, pos):\n    return 1 // 0":                            "strategy.star:2",
		"def on_bar(bar, pos):\n    return \"buy\"":                           "must return",
		"def on_bar(bar, pos):\n    return ema(0)":                            "period must be",
		"def on_bar(bar, pos):\n    for i in range(100000000):\n        pass": "too many steps",
	}
	for src, want := range cases {
		def, err := Define(ScriptType, src)
		require.NoError(t, err, src)
		s, _, err := def.New(nil)
		require.NoError(t, err)

		bar := Bar{Time: time.Unix(0, 0), Open: 1, High: 1, Low: 1, Close: 1}
		assert.Equal(t, Hold, s.OnBar(bar, nil).Action)
		require.Error(t, s.(Failer).Err(), src)
		assert.Contains(t, s.(Failer).Err().Error(), want, src)
		assert.True(t, strings.HasPrefix(s.(Explainer).Explain(), "script error:"))
		assert.Equal(t, Hold, s.OnBar(bar, nil).Action, "it stays stopped")
	}
}

func TestScriptType_IsListedWithAnEmptyParamList(t *testing.T) {
	for _, d := range Definitions() {
		assert.NotNil(t, d.Params, "%s: the UI iterates over params", d.Type)
	}
	def, err := Lookup(ScriptType)
	require.NoError(t, err)
	assert.True(t, def.Scripted)
}

func TestDefine_BuiltInTypesIgnoreTheScript(t *testing.T) {
	def, err := Define("ema_cross", "not a script")
	require.NoError(t, err)
	assert.Equal(t, "ema_cross", def.Type)
	assert.False(t, def.Scripted)

	_, err = Define("no_such_type", "")
	assert.Error(t, err)
}

// The scripts shown to users (the manual and the editor's template) must
// compile and run: a broken example is worse than none.
func TestScript_PublishedExamplesRun(t *testing.T) {
	var sources []string

	manual, err := os.ReadFile("../../../docs/strategy-scripts.md")
	require.NoError(t, err)
	for _, block := range regexp.MustCompile("(?s)```python\n(.*?)```").FindAllStringSubmatch(string(manual), -1) {
		// Fragments that only illustrate one line are completed with "..." in the manual.
		if strings.Contains(block[1], "def on_bar") && !strings.Contains(block[1], "...") {
			sources = append(sources, block[1])
		}
	}
	require.GreaterOrEqual(t, len(sources), 2, "the manual's examples were found")

	template, err := os.ReadFile("../../../apps/web/src/lib/scriptTemplate.ts")
	require.NoError(t, err)
	match := regexp.MustCompile("(?s)SCRIPT_TEMPLATE = `(.*?)`;").FindStringSubmatch(string(template))
	require.NotNil(t, match)
	sources = append(sources, match[1])

	bars := wave(400)
	traded := 0
	for _, src := range sources {
		def, err := Define(ScriptType, src)
		require.NoError(t, err, src)
		s, _, err := def.New(nil)
		require.NoError(t, err)
		for _, sig := range run(s, bars) {
			if sig.Action != Hold {
				traded++
			}
		}
		require.NoError(t, s.(Failer).Err(), src)
	}
	assert.Greater(t, traded, 5, "the examples do trade on a moving market")
}
