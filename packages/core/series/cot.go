package series

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// CFTC Commitments of Traders: how speculators (non-commercial traders) are
// positioned in the currency futures on the CME, published every week.
// Evaluation: docs/backtests/2026-10-27-longterm-donchian-cot.md.
//
// For each currency two series are stored:
//
//	cot_<cur>_net  (long − short) / open interest of speculators in that
//	               currency's futures, e.g. cot_jpy_net > 0 means they buy yen
//	cot_<cur>_pct  where cot_<cur>_net stands among the last COTWindow weeks,
//	               0–100 (0 = most short the currency, 100 = most long)
const (
	// COTURL is the CFTC public API for the legacy futures-only report.
	COTURL = "https://publicreporting.cftc.gov/resource/6dca-aqww.json"
	// COTWindow is the number of weeks (3 years) cot_<cur>_pct compares with.
	COTWindow = 156
	// COTMinWeeks is the fewest weeks cot_<cur>_pct is computed from.
	COTMinWeeks = 52
	// COTDelay is when a report can be used: positions are as of Tuesday and
	// published on Friday afternoon (US), so from the following Monday.
	COTDelay = 6 * 24 * time.Hour
)

// COTMarkets maps a currency (as used in series names) to its CFTC contract
// market code.
var COTMarkets = map[string]string{
	"jpy": "097741",
	"eur": "099741",
	"gbp": "096742",
	"aud": "232741",
}

// COTReport is one week of speculators' positions.
type COTReport struct {
	Date         time.Time // the Tuesday the positions are as of
	Long, Short  float64
	OpenInterest float64
}

// FetchCOT downloads every report for a contract market code, oldest first.
func FetchCOT(ctx context.Context, client *http.Client, baseURL, code string) ([]COTReport, error) {
	q := url.Values{}
	q.Set("$select", "report_date_as_yyyy_mm_dd,noncomm_positions_long_all,noncomm_positions_short_all,open_interest_all")
	q.Set("$where", fmt.Sprintf("cftc_contract_market_code='%s'", code))
	q.Set("$order", "report_date_as_yyyy_mm_dd")
	q.Set("$limit", "10000")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cot %s: %w", code, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cot %s: HTTP %d", code, resp.StatusCode)
	}
	var rows []struct {
		Date         string `json:"report_date_as_yyyy_mm_dd"`
		Long         string `json:"noncomm_positions_long_all"`
		Short        string `json:"noncomm_positions_short_all"`
		OpenInterest string `json:"open_interest_all"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("cot %s: %w", code, err)
	}

	seen := map[time.Time]bool{}
	out := make([]COTReport, 0, len(rows))
	for _, row := range rows {
		date, err := time.Parse("2006-01-02T15:04:05.000", row.Date)
		if err != nil {
			return nil, fmt.Errorf("cot %s: date %q: %w", code, row.Date, err)
		}
		r := COTReport{Date: date}
		for _, f := range []struct {
			raw string
			dst *float64
		}{{row.Long, &r.Long}, {row.Short, &r.Short}, {row.OpenInterest, &r.OpenInterest}} {
			if *f.dst, err = strconv.ParseFloat(f.raw, 64); err != nil {
				return nil, fmt.Errorf("cot %s %s: %w", code, row.Date, err)
			}
		}
		if r.OpenInterest <= 0 || seen[date] {
			continue
		}
		seen[date] = true
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

// COTSeries turns reports (oldest first) into the net and pct series. Each
// week's pct uses only that week and the weeks before it.
func COTSeries(reports []COTReport) (net, pct []Point) {
	values := make([]float64, len(reports))
	for i, r := range reports {
		values[i] = (r.Long - r.Short) / r.OpenInterest
		at := r.Date.Add(COTDelay)
		net = append(net, Point{Time: at, Value: values[i]})

		window := values[max(0, i-COTWindow+1) : i+1]
		if len(window) < COTMinWeeks {
			continue
		}
		below := 0
		for _, v := range window[:len(window)-1] {
			if v < values[i] {
				below++
			}
		}
		pct = append(pct, Point{Time: at, Value: float64(below) / float64(len(window)-1) * 100})
	}
	return net, pct
}

// SyncCOT downloads the reports for every currency in COTMarkets and stores
// both series of each.
func SyncCOT(ctx context.Context, client *http.Client, baseURL string, store MySQLStore) (map[string]int, error) {
	weeks := map[string]int{}
	for cur, code := range COTMarkets {
		reports, err := FetchCOT(ctx, client, baseURL, code)
		if err != nil {
			return weeks, err
		}
		net, pct := COTSeries(reports)
		if err := store.Replace(ctx, "cot_"+cur+"_net", net); err != nil {
			return weeks, err
		}
		if err := store.Replace(ctx, "cot_"+cur+"_pct", pct); err != nil {
			return weeks, err
		}
		weeks[cur] = len(reports)
	}
	return weeks, nil
}
