package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/core/broker/gmofx"
	"github.com/moomoo-trading/core/market"
)

// historyStart is the first day GMO serves bars for.
var historyStart = time.Date(2023, 10, 28, 0, 0, 0, 0, time.UTC)

// MarketDataHandler serves the instruments, the stored price history and the
// requests to download more.
type MarketDataHandler struct {
	repo        *database.MarketDataRepository
	instruments *instrumentCache
	now         func() time.Time
}

func NewMarketDataHandler(repo *database.MarketDataRepository, instruments instrumentSource) *MarketDataHandler {
	return &MarketDataHandler{repo: repo, instruments: &instrumentCache{src: instruments, now: time.Now}, now: time.Now}
}

// Get returns everything the market data screen and the backtest form show.
func (h *MarketDataHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	instruments, err := h.instruments.list(ctx)
	if err != nil {
		// The stored data is still worth showing while the broker is unreachable.
		instruments = []tradable{}
	}
	coverage, err := h.repo.Coverage(ctx, gmofx.BrokerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存済みのデータを取得できませんでした"})
		return
	}
	imports, err := h.repo.Imports(ctx, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取り込みの履歴を取得できませんでした"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"instruments":   instruments,
		"coverage":      coverage,
		"imports":       imports,
		"history_start": historyStart.Format("2006-01-02"),
	}})
}

// CreateImport queues a download of BID and ASK bars; the bot carries it out.
func (h *MarketDataHandler) CreateImport(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		Symbol    string `json:"symbol"`
		Timeframe string `json:"timeframe"`
		From      string `json:"from"` // YYYY-MM-DD; defaults to the start of GMO's history
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "リクエストの形式が正しくありません"})
		return
	}
	if _, err := market.Timeframe(req.Timeframe).Duration(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "足の指定が正しくありません"})
		return
	}
	from := historyStart
	if req.From != "" {
		parsed, err := time.Parse("2006-01-02", req.From)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "開始日は YYYY-MM-DD の形で指定してください"})
			return
		}
		from = parsed
	}
	if from.Before(historyStart) {
		from = historyStart // nothing older exists
	}
	if !from.Before(h.now().UTC()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "開始日は今日より前にしてください"})
		return
	}

	instruments, err := h.instruments.list(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "ブローカーから銘柄の情報を取得できませんでした"})
		return
	}
	known := false
	for _, in := range instruments {
		known = known || in.Symbol == req.Symbol
	}
	if !known {
		c.JSON(http.StatusBadRequest, gin.H{"error": "この銘柄は取り込めません（円建ての通貨ペアのみ対応）"})
		return
	}

	id, err := h.repo.CreateImport(ctx, gmofx.BrokerName, req.Symbol, req.Timeframe, from)
	if errors.Is(err, database.ErrImportQueued) {
		c.JSON(http.StatusConflict, gin.H{"error": "同じ銘柄・足の取り込みが、すでに待機中または実行中です"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取り込みを登録できませんでした"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

// CancelImport withdraws a download that has not started yet.
func (h *MarketDataHandler) CancelImport(c *gin.Context) {
	cancelled, err := h.repo.CancelImport(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取り込みを取り消せませんでした"})
		return
	}
	if !cancelled {
		c.JSON(http.StatusConflict, gin.H{"error": "待機中の取り込みではないため、取り消せません"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id")}})
}
