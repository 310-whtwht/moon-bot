package tradescmd

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteComputesSlippageAndR(t *testing.T) {
	at := time.Date(2026, 11, 2, 1, 0, 5, 0, time.UTC)
	rows := []Row{
		{ID: "p1", Deployment: "d", Version: "v1", Broker: "paper", Account: "default", Symbol: "USD_JPY", Side: "buy",
			OpenedAt: at, ClosedAt: at.Add(time.Hour), Units: 100, OpenPrice: 150.012, ClosePrice: 149.495,
			Fees: 0.6, PnL: -52.3, ExitReason: "stop",
			ExpectedOpen:  sql.NullFloat64{Float64: 150.010, Valid: true},
			ExpectedClose: sql.NullFloat64{Float64: 149.5, Valid: true},
			InitialStop:   sql.NullFloat64{Float64: 149.512, Valid: true}},
		// Recorded before the columns existed, and not yen-quoted: left blank.
		{ID: "p2", Broker: "paper", Symbol: "EUR_USD", Side: "sell", OpenedAt: at, ClosedAt: at,
			Units: 100, OpenPrice: 1.1, ClosePrice: 1.09, PnL: 150,
			InitialStop: sql.NullFloat64{Float64: 1.11, Valid: true}},
	}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, rows))
	recs, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, recs, 3)
	col := map[string]int{}
	for i, h := range recs[0] {
		col[h] = i
	}
	r1 := recs[1]
	assert.Equal(t, "0.002000", r1[col["entry_slippage"]])
	assert.Equal(t, "0.005000", r1[col["exit_slippage"]])
	assert.Equal(t, "50.00", r1[col["risk_jpy"]])
	assert.Equal(t, "-1.05", r1[col["r_multiple"]])
	assert.Equal(t, "stop", r1[col["exit_reason"]])

	r2 := recs[2]
	for _, c := range []string{"expected_entry_price", "entry_slippage", "exit_slippage", "risk_jpy", "r_multiple", "exit_reason"} {
		assert.Empty(t, r2[col[c]], c)
	}
}
