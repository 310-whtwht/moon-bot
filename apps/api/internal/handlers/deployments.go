package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/core/market"
)

// instrumentSource lists what the broker lets us trade.
type instrumentSource interface {
	Instruments(ctx context.Context) ([]market.Instrument, error)
}

// tradable is an instrument a deployment may use.
type tradable struct {
	Symbol   string  `json:"symbol"`
	Quote    string  `json:"quote"` // currency the price is in, e.g. JPY
	MinUnits float64 `json:"min_units"`
	Step     float64 `json:"step"`
}

// instrumentCache keeps the broker's instrument list for an hour.
type instrumentCache struct {
	src instrumentSource
	now func() time.Time

	mu      sync.Mutex
	loaded  time.Time
	entries []tradable
}

// all returns every instrument the broker offers.
func (c *instrumentCache) all(ctx context.Context) ([]tradable, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries != nil && c.now().Sub(c.loaded) < time.Hour {
		return c.entries, nil
	}
	instruments, err := c.src.Instruments(ctx)
	if err != nil {
		if c.entries != nil {
			return c.entries, nil // a stale list beats none
		}
		return nil, err
	}
	entries := []tradable{}
	for _, in := range instruments {
		entries = append(entries, tradable{
			Symbol:   in.Key.Symbol,
			Quote:    in.QuoteCurrency,
			MinUnits: in.MinOrderSize.InexactFloat64(),
			Step:     in.SizeStep.InexactFloat64(),
		})
	}
	c.entries, c.loaded = entries, c.now()
	return entries, nil
}

// list returns the instruments quoted in JPY: the ones a deployment may
// trade. Live profit, loss and the risk limits are all counted in yen, so
// other pairs cannot be deployed yet (they can be downloaded and backtested).
func (c *instrumentCache) list(ctx context.Context) ([]tradable, error) {
	all, err := c.all(ctx)
	if err != nil {
		return nil, err
	}
	out := []tradable{}
	for _, t := range all {
		if t.Quote == "JPY" {
			out = append(out, t)
		}
	}
	return out, nil
}

// checkUnits validates an order size against an instrument.
func (t tradable) checkUnits(units float64) error {
	if units < t.MinUnits {
		return fmt.Errorf("%s は %.0f 通貨以上で指定してください", t.Symbol, t.MinUnits)
	}
	if t.Step > 0 {
		if steps := units / t.Step; math.Abs(steps-math.Round(steps)) > 1e-9 {
			return fmt.Errorf("%s の数量は %.0f 通貨単位で指定してください", t.Symbol, t.Step)
		}
	}
	return nil
}

func (h *BotHandler) tradable(ctx context.Context, symbol string) (tradable, bool, error) {
	list, err := h.instruments.list(ctx)
	if err != nil {
		return tradable{}, false, err
	}
	for _, t := range list {
		if t.Symbol == symbol {
			return t, true, nil
		}
	}
	return tradable{}, false, nil
}

// entryRequest is how a deployment sends its entries, as the UI submits it.
type entryRequest struct {
	Order       string   `json:"entry_order"`
	WaitSeconds *int     `json:"limit_wait_seconds"`
	Fallback    string   `json:"limit_fallback"`
	MaxSpread   *float64 `json:"max_spread"`
}

const (
	minLimitWait = 5
	maxLimitWait = 300
)

// settings validates the request and fills in the defaults.
func (e entryRequest) settings() (database.EntrySettings, error) {
	out := database.EntrySettings{Order: e.Order, Fallback: e.Fallback, WaitSeconds: 30}
	if out.Order == "" {
		out.Order = "market"
	}
	if out.Fallback == "" {
		out.Fallback = "skip"
	}
	if out.Order != "market" && out.Order != "limit" {
		return out, errors.New("発注方法は market か limit で指定してください")
	}
	if out.Fallback != "skip" && out.Fallback != "market" {
		return out, errors.New("指値が約定しなかったときの動作は skip か market で指定してください")
	}
	if e.WaitSeconds != nil {
		out.WaitSeconds = *e.WaitSeconds
	}
	if out.WaitSeconds < minLimitWait || out.WaitSeconds > maxLimitWait {
		return out, fmt.Errorf("指値の待ち時間は %d〜%d 秒で指定してください", minLimitWait, maxLimitWait)
	}
	if e.MaxSpread != nil {
		if *e.MaxSpread < 0 || math.IsNaN(*e.MaxSpread) {
			return out, errors.New("スプレッドの上限は 0 以上で指定してください")
		}
		if *e.MaxSpread > 0 { // 0 means no limit
			out.MaxSpread = e.MaxSpread
		}
	}
	return out, nil
}

// GetInstruments lists the instruments a deployment may trade.
func (h *BotHandler) GetInstruments(c *gin.Context) {
	list, err := h.instruments.list(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to load instruments from the broker"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// CreateDeployment adds a deployment on the paper account. It starts
// disabled: trading begins when it is switched on.
func (h *BotHandler) CreateDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		Name       string  `json:"name"`
		StrategyID string  `json:"strategy_id"`
		Symbol     string  `json:"symbol"`
		Timeframe  string  `json:"timeframe"`
		Units      float64 `json:"units"`
		entryRequest
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "リクエストの形式が正しくありません"})
		return
	}
	entry, err := req.entryRequest.settings()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名前を入力してください"})
		return
	}
	if _, err := market.Timeframe(req.Timeframe).Duration(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "足の指定が正しくありません"})
		return
	}
	instrument, ok, err := h.tradable(ctx, req.Symbol)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "ブローカーから銘柄の情報を取得できませんでした"})
		return
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "この銘柄は割り当てできません（円建ての通貨ペアのみ対応）"})
		return
	}
	if err := instrument.checkUnits(req.Units); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if found, err := h.repo.StrategyExists(ctx, req.StrategyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "戦略を確認できませんでした"})
		return
	} else if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "戦略が見つかりません"})
		return
	}

	id, err := h.repo.CreateDeployment(ctx, database.NewDeployment{
		Name: req.Name, StrategyID: req.StrategyID, Broker: "paper", AccountID: "default",
		Symbol: req.Symbol, Timeframe: req.Timeframe, Units: req.Units, Entry: entry,
	})
	if errors.Is(err, database.ErrDeploymentTaken) {
		c.JSON(http.StatusConflict, gin.H{"error": "この銘柄の割り当ては既にあります（1口座につき1銘柄1つ）"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "割り当てを作成できませんでした"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

// UpdateDeployment changes a deployment's name, size, enabled state or entry settings.
// Disabling does not close an open position: the bot keeps managing its exit
// and stop-loss. A new size applies from the next entry.
func (h *BotHandler) UpdateDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	var req struct {
		Name    *string  `json:"name"`
		Units   *float64 `json:"units"`
		Enabled *bool    `json:"enabled"`
		// Entry settings are replaced together, when entry_order is given.
		entryRequest
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.Name == nil && req.Units == nil && req.Enabled == nil && req.entryRequest.Order == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name・units・enabled・entry_order のいずれかを指定してください"})
		return
	}
	var entry *database.EntrySettings
	if req.entryRequest.Order != "" {
		settings, err := req.entryRequest.settings()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		entry = &settings
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "名前を入力してください"})
			return
		}
		req.Name = &name
	}
	if req.Units != nil {
		symbol, err := h.repo.DeploymentSymbol(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "割り当てを確認できませんでした"})
			return
		}
		if symbol == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "割り当てが見つかりません"})
			return
		}
		instrument, ok, err := h.tradable(ctx, symbol)
		if err != nil || !ok {
			c.JSON(http.StatusBadGateway, gin.H{"error": "ブローカーから銘柄の情報を取得できませんでした"})
			return
		}
		if err := instrument.checkUnits(*req.Units); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	found, err := h.repo.UpdateDeployment(ctx, id, database.DeploymentPatch{Name: req.Name, Units: req.Units, Enabled: req.Enabled, Entry: entry})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "割り当てを更新できませんでした"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "割り当てが見つかりません"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// DeleteDeployment removes a deployment that holds no position.
func (h *BotHandler) DeleteDeployment(c *gin.Context) {
	found, err := h.repo.DeleteDeployment(c.Request.Context(), c.Param("id"))
	if errors.Is(err, database.ErrDeploymentInUse) {
		c.JSON(http.StatusConflict, gin.H{"error": "建玉を保有中、または指値が待機中のため削除できません。無効にして、決済（または取消）されてから削除してください"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "割り当てを削除できませんでした"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "割り当てが見つかりません"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id")}})
}
