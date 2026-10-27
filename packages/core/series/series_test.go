package series

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCache_ReturnsTheLatestPublicValue(t *testing.T) {
	loads := 0
	now := day("2026-10-01")
	c := &Cache{
		TTL: time.Hour,
		Now: func() time.Time { return now },
		Load: func(_ context.Context, name string) ([]Point, error) {
			loads++
			if name != "x" {
				return nil, nil
			}
			return []Point{{day("2026-01-05"), 1}, {day("2026-01-12"), 2}}, nil
		},
	}

	_, ok := c.At("x", day("2026-01-04"))
	assert.False(t, ok, "nothing public yet")
	v, ok := c.At("x", day("2026-01-05"))
	assert.True(t, ok)
	assert.Equal(t, 1.0, v, "public from its time on")
	v, _ = c.At("x", day("2026-01-11"))
	assert.Equal(t, 1.0, v)
	v, _ = c.At("x", day("2027-01-01"))
	assert.Equal(t, 2.0, v)
	_, ok = c.At("unknown", day("2027-01-01"))
	assert.False(t, ok)
	assert.Equal(t, 2, loads, "each series is loaded once")

	now = now.Add(2 * time.Hour)
	c.At("x", day("2027-01-01"))
	assert.Equal(t, 3, loads, "reloaded after TTL")
}

func TestCache_KeepsTheLastCopyWhenAReloadFails(t *testing.T) {
	now := day("2026-10-01")
	fail := false
	c := &Cache{
		TTL: time.Hour,
		Now: func() time.Time { return now },
		Load: func(context.Context, string) ([]Point, error) {
			if fail {
				return nil, errors.New("db down")
			}
			return []Point{{day("2026-01-05"), 1}}, nil
		},
	}
	c.At("x", now)
	fail, now = true, now.Add(2*time.Hour)
	v, ok := c.At("x", now)
	assert.True(t, ok)
	assert.Equal(t, 1.0, v)
}

func TestCOTSeries(t *testing.T) {
	var reports []COTReport
	start := day("2020-01-07")
	for i := 0; i < COTMinWeeks+2; i++ {
		// net rises by 1% a week, except the last week falls back to the start
		long := 100.0 + float64(i)
		if i == COTMinWeeks+1 {
			long = 100
		}
		reports = append(reports, COTReport{Date: start.AddDate(0, 0, 7*i), Long: long, Short: 100, OpenInterest: 100})
	}
	net, pct := COTSeries(reports)

	require.Len(t, net, len(reports))
	assert.Equal(t, start.Add(COTDelay), net[0].Time, "usable from the Monday after")
	assert.InDelta(t, 0.0, net[0].Value, 1e-12)
	assert.InDelta(t, 0.05, net[5].Value, 1e-12)

	require.Len(t, pct, 3, "from the 52nd week on")
	assert.Equal(t, net[COTMinWeeks-1].Time, pct[0].Time)
	assert.Equal(t, 100.0, pct[0].Value, "the highest so far")
	assert.Equal(t, 100.0, pct[1].Value)
	assert.Equal(t, 0.0, pct[2].Value, "the lowest: equal values do not count as below")
}

func TestFetchCOT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "cftc_contract_market_code='097741'", r.URL.Query().Get("$where"))
		_, _ = w.Write([]byte(`[
			{"report_date_as_yyyy_mm_dd":"2026-10-20T00:00:00.000","noncomm_positions_long_all":"200186","noncomm_positions_short_all":"130308","open_interest_all":"379141"},
			{"report_date_as_yyyy_mm_dd":"2026-10-13T00:00:00.000","noncomm_positions_long_all":"1","noncomm_positions_short_all":"2","open_interest_all":"10"},
			{"report_date_as_yyyy_mm_dd":"2026-10-13T00:00:00.000","noncomm_positions_long_all":"9","noncomm_positions_short_all":"9","open_interest_all":"10"}
		]`))
	}))
	defer srv.Close()

	got, err := FetchCOT(context.Background(), srv.Client(), srv.URL, "097741")
	require.NoError(t, err)
	assert.Equal(t, []COTReport{
		{Date: day("2026-10-13"), Long: 1, Short: 2, OpenInterest: 10},
		{Date: day("2026-10-20"), Long: 200186, Short: 130308, OpenInterest: 379141},
	}, got, "sorted, one report per date")
}
