package strategy

// StrategyTemplates contains predefined strategy templates
var StrategyTemplates = map[string]string{
	"ema_cross": `
# EMA Cross Strategy
# Parameters: fast_period, slow_period, quantity

def on_bar(symbol, bar):
    fast_ema = ema(bar.close, fast_period)
    slow_ema = ema(bar.close, slow_period)
    
    if fast_ema > slow_ema and position(symbol) <= 0:
        order(symbol, "BUY", "MARKET", quantity)
        log("EMA Cross: BUY signal for " + symbol)
    elif fast_ema < slow_ema and position(symbol) >= 0:
        order(symbol, "SELL", "MARKET", quantity)
        log("EMA Cross: SELL signal for " + symbol)
`,

	"rsi_reversal": `
# RSI Reversal Strategy
# Parameters: rsi_period, oversold, overbought, quantity

def on_bar(symbol, bar):
    rsi_val = rsi(bar.close, rsi_period)
    
    if rsi_val < oversold and position(symbol) <= 0:
        order(symbol, "BUY", "MARKET", quantity)
        log("RSI Reversal: BUY signal for " + symbol + " (RSI: " + str(rsi_val) + ")")
    elif rsi_val > overbought and position(symbol) >= 0:
        order(symbol, "SELL", "MARKET", quantity)
        log("RSI Reversal: SELL signal for " + symbol + " (RSI: " + str(rsi_val) + ")")
`,

	"breakout": `
# Breakout Strategy
# Parameters: lookback_period, quantity

def on_bar(symbol, bar):
    high = highest(bar.high, lookback_period)
    low = lowest(bar.low, lookback_period)
    
    if bar.close > high and position(symbol) <= 0:
        order(symbol, "BUY", "MARKET", quantity)
        log("Breakout: BUY signal for " + symbol + " (Breakout above " + str(high) + ")")
    elif bar.close < low and position(symbol) >= 0:
        order(symbol, "SELL", "MARKET", quantity)
        log("Breakout: SELL signal for " + symbol + " (Breakout below " + str(low) + ")")
`,

	"ichimoku": `
# Ichimoku Strategy
# Parameters: quantity

def on_bar(symbol, bar):
    tenkan = ichimoku_tenkan(bar)
    kijun = ichimoku_kijun(bar)
    senkou_a = ichimoku_senkou_a(bar)
    senkou_b = ichimoku_senkou_b(bar)
    
    # Price above cloud and tenkan above kijun
    if (bar.close > senkou_a and bar.close > senkou_b and 
        tenkan > kijun and position(symbol) <= 0):
        order(symbol, "BUY", "MARKET", quantity)
        log("Ichimoku: BUY signal for " + symbol)
    # Price below cloud and tenkan below kijun
    elif (bar.close < senkou_a and bar.close < senkou_b and 
          tenkan < kijun and position(symbol) >= 0):
        order(symbol, "SELL", "MARKET", quantity)
        log("Ichimoku: SELL signal for " + symbol)
`,

	"mean_reversion": `
# Mean Reversion Strategy
# Parameters: lookback_period, std_dev, quantity

def on_bar(symbol, bar):
    mean = sma(bar.close, lookback_period)
    std = stddev(bar.close, lookback_period)
    
    upper_band = mean + (std_dev * std)
    lower_band = mean - (std_dev * std)
    
    if bar.close < lower_band and position(symbol) <= 0:
        order(symbol, "BUY", "MARKET", quantity)
        log("Mean Reversion: BUY signal for " + symbol + " (Price: " + str(bar.close) + " < " + str(lower_band) + ")")
    elif bar.close > upper_band and position(symbol) >= 0:
        order(symbol, "SELL", "MARKET", quantity)
        log("Mean Reversion: SELL signal for " + symbol + " (Price: " + str(bar.close) + " > " + str(upper_band) + ")")
`,

	"momentum": `
# Momentum Strategy
# Parameters: lookback_period, threshold, quantity

def on_bar(symbol, bar):
    momentum = (bar.close - bar.close[lookback_period]) / bar.close[lookback_period] * 100
    
    if momentum > threshold and position(symbol) <= 0:
        order(symbol, "BUY", "MARKET", quantity)
        log("Momentum: BUY signal for " + symbol + " (Momentum: " + str(momentum) + "%)")
    elif momentum < -threshold and position(symbol) >= 0:
        order(symbol, "SELL", "MARKET", quantity)
        log("Momentum: SELL signal for " + symbol + " (Momentum: " + str(momentum) + "%)")
`,
}

// GetTemplate returns a strategy template by name
func GetTemplate(name string) (string, bool) {
	template, exists := StrategyTemplates[name]
	return template, exists
}

// ListTemplates returns all available template names
func ListTemplates() []string {
	templates := make([]string, 0, len(StrategyTemplates))
	for name := range StrategyTemplates {
		templates = append(templates, name)
	}
	return templates
}