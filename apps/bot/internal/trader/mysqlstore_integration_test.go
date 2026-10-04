package trader

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/moomoo-trading/core/risk"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLStore_EndToEnd runs the trader on a migrated database when
// MOONBOT_TEST_DSN is set (use loc=UTC), with the paper broker and a fake feed.
func TestMySQLStore_EndToEnd(t *testing.T) {
	dsn := os.Getenv("MOONBOT_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOT_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	const (
		strategyID = "e2e00000-0000-0000-0000-000000000001"
		versionID  = "e2e00000-0000-0000-0000-000000000002"
		deployID   = "e2e00000-0000-0000-0000-000000000003"
		account    = "e2e-test"
	)
	cleanup := func() {
		for _, q := range []string{
			`DELETE FROM trades WHERE account_id = '` + account + `'`,
			`DELETE FROM orders WHERE account_id = '` + account + `'`,
			`DELETE FROM positions WHERE account_id = '` + account + `'`,
			`DELETE FROM deployments WHERE id = '` + deployID + `'`,
			`DELETE FROM strategy_packages WHERE id = '` + strategyID + `'`, // cascades to versions and params
		} {
			_, err := db.ExecContext(ctx, q)
			require.NoError(t, err)
		}
	}
	cleanup()
	defer cleanup()

	exec := func(q string, args ...any) {
		_, err := db.ExecContext(ctx, q, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO strategy_packages (id, name, author) VALUES (?, 'e2e', 'test')`, strategyID)
	exec(`INSERT INTO strategy_versions (id, package_id, version, code, is_active) VALUES (?, ?, '1', 'test_scripted', TRUE)`, versionID, strategyID)
	exec(`INSERT INTO strategy_params (id, version_id, param_name, param_type, default_value) VALUES (UUID(), ?, 'variant', 'number', '0')`, versionID)
	// A dedicated account keeps clear of the seeded deployment's unique (broker, account, symbol).
	exec(`INSERT INTO deployments (id, name, strategy_id, broker, account_id, symbol, timeframe, units, enabled)
VALUES (?, 'e2e', ?, 'paper', ?, 'USD_JPY', '1h', 100, TRUE)`, deployID, strategyID, account)

	store := &MySQLStore{DB: db}
	deployments, err := store.Deployments(ctx)
	require.NoError(t, err)
	var found bool
	for _, dp := range deployments {
		found = found || (dp.ID == deployID && dp.Enabled && dp.Units.String() == "100")
	}
	require.True(t, found)

	v, err := store.ActiveVersion(ctx, strategyID)
	require.NoError(t, err)
	assert.Equal(t, Version{ID: versionID, Type: "test_scripted", Params: strategy.Params{"variant": 0}}, v)

	f := newFeed()
	pb := paper.New(f, paper.Options{AccountID: account, InitialBalance: d("30000"), FeeRate: d("0.00002"), Now: f.clock})
	mgr := &Manager{
		Store: store, Brokers: map[string]broker.Broker{paper.BrokerName: pb}, Now: f.clock,
		Config: Config{AccountLimits: risk.Limits{MaxDailyLossJPY: 100}},
		Logf:   t.Logf,
	}
	// Only run our deployment: other rows in a shared database are not ours to trade.
	mgr.Store = onlyDeployment{Store: store, id: deployID}

	setScript(0, map[int]strategy.Signal{10: long(149), 12: long(148)})
	f.at(10)
	require.NoError(t, mgr.PollOnce(ctx))
	f.at(11)
	require.NoError(t, mgr.PollOnce(ctx))

	pos, err := store.OpenPosition(ctx, deployID)
	require.NoError(t, err)
	require.NotNil(t, pos)
	assert.Equal(t, broker.SideBuy, pos.Side)
	assert.Equal(t, "150.01", pos.OpenPrice.String())
	assert.Equal(t, "149", pos.StopPrice.String())
	assert.Equal(t, versionID, pos.StrategyVersionID)
	assert.Equal(t, time.UTC, pos.OpenedAt.Location())

	// The same decision cannot be recorded twice.
	created, err := store.CreateOrder(ctx, Order{ClientOrderID: fmt.Sprintf("e2e00000-%d-open", t0.Add(10*time.Hour).Unix()), Broker: "paper",
		AccountID: account, Symbol: "USD_JPY", Side: broker.SideBuy, SettleType: "open", Units: d("100")})
	require.NoError(t, err)
	assert.False(t, created)

	// Restart: a fresh paper broker gets its state back from the database.
	pb2 := paper.New(f, paper.Options{AccountID: account, InitialBalance: d("30000"), FeeRate: d("0.00002"), Now: f.clock})
	require.NoError(t, RestorePaper(ctx, store, pb2, d("30000")))
	a1, _ := pb.Assets(ctx)
	a2, _ := pb2.Assets(ctx)
	assert.Equal(t, a1.Equity.String(), a2.Equity.String())
	assert.Equal(t, a1.AvailableMargin.String(), a2.AvailableMargin.String())

	// Stop out, then the daily loss limit rejects the next entry.
	f.quote("148.900", "148.910")
	mgr.OnTick(ctx, f.tick())
	pos, err = store.OpenPosition(ctx, deployID)
	require.NoError(t, err)
	assert.Nil(t, pos)

	realized, err := store.RealizedPnL(ctx, "paper", account)
	require.NoError(t, err)
	assert.Equal(t, "-111.59782", realized.String())

	acct, global, err := store.Exposure(ctx, "paper", account, risk.TradingDayStart(f.clock()), risk.TradingWeekStart(f.clock()))
	require.NoError(t, err)
	assert.Equal(t, 0, acct.OpenPositions)
	assert.InDelta(t, -111.59782, acct.DailyPnLJPY, 1e-6)
	assert.InDelta(t, -111.59782, acct.WeeklyPnLJPY, 1e-6)
	assert.LessOrEqual(t, global.DailyPnLJPY, acct.DailyPnLJPY+1e-6, "global includes this account")

	f.quote("150.000", "150.010")
	f.at(13)
	require.NoError(t, mgr.PollOnce(ctx))

	// created_at has second precision, so compare without relying on row order.
	rows, err := db.QueryContext(ctx,
		`SELECT settle_type, status, COALESCE(error_message, '') FROM orders WHERE account_id = ?`, account)
	require.NoError(t, err)
	defer rows.Close()
	var got []string
	var rejection string
	for rows.Next() {
		var settle, status, reason string
		require.NoError(t, rows.Scan(&settle, &status, &reason))
		got = append(got, settle+":"+status)
		if status == "rejected" {
			rejection = reason
		}
	}
	assert.ElementsMatch(t, []string{"open:filled", "close:filled", "open:rejected"}, got)
	assert.Contains(t, rejection, "daily_loss")

	var trades int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM trades WHERE account_id = ?`, account).Scan(&trades))
	assert.Equal(t, 2, trades, "one trade per fill")
}

// onlyDeployment narrows Deployments to one row.
type onlyDeployment struct {
	Store
	id string
}

func (o onlyDeployment) Deployments(ctx context.Context) ([]Deployment, error) {
	all, err := o.Store.Deployments(ctx)
	if err != nil {
		return nil, err
	}
	var out []Deployment
	for _, d := range all {
		if d.ID == o.id {
			out = append(out, d)
		}
	}
	return out, nil
}
