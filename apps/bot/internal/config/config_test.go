package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLiveTradingNeedsEverySwitch(t *testing.T) {
	full := func() *Config {
		return &Config{
			GMO:    GMOConfig{APIKey: "k", APISecret: "s"},
			Trader: TraderConfig{BrokerMode: "live", LiveConfirm: "yes"},
		}
	}

	ok, why := full().LiveTrading()
	assert.True(t, ok)
	assert.Empty(t, why)

	cases := map[string]func(*Config){
		"default mode":      func(c *Config) { c.Trader.BrokerMode = "paper" },
		"empty mode":        func(c *Config) { c.Trader.BrokerMode = "" },
		"no confirmation":   func(c *Config) { c.Trader.LiveConfirm = "" },
		"wrong confirm":     func(c *Config) { c.Trader.LiveConfirm = "true" },
		"missing key":       func(c *Config) { c.GMO.APIKey = "" },
		"missing secret":    func(c *Config) { c.GMO.APISecret = "" },
		"mode is uppercase": func(c *Config) { c.Trader.BrokerMode = "LIVE" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := full()
			mutate(c)
			ok, why := c.LiveTrading()
			assert.False(t, ok)
			assert.NotEmpty(t, why)
		})
	}
}

func TestLoadDefaultsToPaper(t *testing.T) {
	t.Setenv("BROKER_MODE", "")
	t.Setenv("LIVE_CONFIRM", "")
	c := Load()
	assert.Equal(t, "paper", c.Trader.BrokerMode)
	ok, _ := c.LiveTrading()
	assert.False(t, ok)
}

func TestGetFloatFallsBackOnBadInput(t *testing.T) {
	t.Setenv("RISK_ACCOUNT_MAX_DAILY_LOSS", "abc")
	assert.Equal(t, 500.0, Load().Trader.AccountLimits.MaxDailyLossJPY, "a typo must not disable the limit")
	t.Setenv("RISK_ACCOUNT_MAX_DAILY_LOSS", "-5")
	assert.Equal(t, 500.0, Load().Trader.AccountLimits.MaxDailyLossJPY)
	t.Setenv("RISK_ACCOUNT_MAX_DAILY_LOSS", "250")
	assert.Equal(t, 250.0, Load().Trader.AccountLimits.MaxDailyLossJPY)
}
