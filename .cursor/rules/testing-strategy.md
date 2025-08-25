# テスト戦略仕様

## 1. テストピラミッド

### 1.1 テスト構成
```
    E2E Tests (10%)
   ┌─────────────┐
   │             │
   │ Integration │ (20%)
   │    Tests    │
   │             │
   └─────────────┘
   ┌─────────────┐
   │             │
   │   Unit      │ (70%)
   │   Tests     │
   │             │
   └─────────────┘
```

### 1.2 テスト実行時間目標
- **Unit Tests**: < 30秒
- **Integration Tests**: < 5分
- **E2E Tests**: < 15分
- **全体**: < 20分

## 2. 単体テスト

### 2.1 Go API 単体テスト
```go
// internal/strategy/engine_test.go
package strategy

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// モック定義
type MockBrokerAdapter struct {
    mock.Mock
}

func (m *MockBrokerAdapter) PlaceOrder(order *Order) error {
    args := m.Called(order)
    return args.Error(0)
}

// テストケース
func TestEMAStrategy(t *testing.T) {
    // セットアップ
    mockBroker := new(MockBrokerAdapter)
    engine := NewStrategyEngine(mockBroker)
    
    // テストデータ準備
    testData := []MarketData{
        {Timestamp: time.Now(), Close: 100.0},
        {Timestamp: time.Now().Add(time.Minute), Close: 101.0},
        // ... テストデータ
    }
    
    // 期待値設定
    mockBroker.On("PlaceOrder", mock.AnythingOfType("*Order")).Return(nil)
    
    // 実行
    result := engine.ExecuteStrategy("ema_cross", testData)
    
    // 検証
    assert.NoError(t, result.Error)
    assert.True(t, result.SignalGenerated)
    mockBroker.AssertExpectations(t)
}

// テーブル駆動テスト
func TestCalculatePositionSize(t *testing.T) {
    tests := []struct {
        name           string
        riskPerTrade   float64
        stopLossPct    float64
        currentPrice   float64
        expectedSize   int
    }{
        {
            name:         "正常なサイズ計算",
            riskPerTrade: 0.0025,
            stopLossPct:  0.02,
            currentPrice: 100.0,
            expectedSize: 125,
        },
        {
            name:         "最小サイズ",
            riskPerTrade: 0.0001,
            stopLossPct:  0.01,
            currentPrice: 1000.0,
            expectedSize: 1,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            size := CalculatePositionSize(tt.riskPerTrade, tt.stopLossPct, tt.currentPrice)
            assert.Equal(t, tt.expectedSize, size)
        })
    }
}

// ベンチマークテスト
func BenchmarkEMAStrategy(b *testing.B) {
    engine := NewStrategyEngine(nil)
    testData := generateTestData(1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.ExecuteStrategy("ema_cross", testData)
    }
}
```

### 2.2 Next.js フロントエンド単体テスト
```typescript
// apps/web/__tests__/components/StrategyCard.test.tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { StrategyCard } from '@/components/StrategyCard'
import { mockStrategy } from '@/__mocks__/strategy'

describe('StrategyCard', () => {
    it('戦略情報を正しく表示する', () => {
        render(<StrategyCard strategy={mockStrategy} />)
        
        expect(screen.getByText(mockStrategy.name)).toBeInTheDocument()
        expect(screen.getByText(mockStrategy.description)).toBeInTheDocument()
        expect(screen.getByText(mockStrategy.category)).toBeInTheDocument()
    })
    
    it('編集ボタンクリックでコールバックが呼ばれる', () => {
        const onEdit = jest.fn()
        render(<StrategyCard strategy={mockStrategy} onEdit={onEdit} />)
        
        fireEvent.click(screen.getByRole('button', { name: /編集/i }))
        expect(onEdit).toHaveBeenCalledWith(mockStrategy.id)
    })
    
    it('削除ボタンクリックで確認ダイアログが表示される', () => {
        const onDelete = jest.fn()
        render(<StrategyCard strategy={mockStrategy} onDelete={onDelete} />)
        
        fireEvent.click(screen.getByRole('button', { name: /削除/i }))
        expect(screen.getByText(/本当に削除しますか/i)).toBeInTheDocument()
    })
})

// カスタムフックのテスト
// apps/web/__tests__/hooks/useStrategies.test.ts
import { renderHook, waitFor } from '@testing-library/react'
import { useStrategies } from '@/hooks/useStrategies'
import { mockApi } from '@/__mocks__/api'

describe('useStrategies', () => {
    it('戦略一覧を取得する', async () => {
        const { result } = renderHook(() => useStrategies())
        
        await waitFor(() => {
            expect(result.current.strategies).toHaveLength(3)
            expect(result.current.loading).toBe(false)
        })
    })
    
    it('エラー時にエラー状態になる', async () => {
        mockApi.getStrategies.mockRejectedValue(new Error('API Error'))
        
        const { result } = renderHook(() => useStrategies())
        
        await waitFor(() => {
            expect(result.current.error).toBeTruthy()
            expect(result.current.loading).toBe(false)
        })
    })
})
```

### 2.3 ユーティリティ関数テスト
```typescript
// apps/web/__tests__/utils/calculations.test.ts
import { calculatePnL, calculateDrawdown, formatCurrency } from '@/utils/calculations'

describe('calculations', () => {
    describe('calculatePnL', () => {
        it('利益を正しく計算する', () => {
            const result = calculatePnL(100, 110, 1000)
            expect(result).toBe(10000) // (110 - 100) * 1000
        })
        
        it('損失を正しく計算する', () => {
            const result = calculatePnL(100, 90, 1000)
            expect(result).toBe(-10000) // (90 - 100) * 1000
        })
        
        it('手数料を考慮する', () => {
            const result = calculatePnL(100, 110, 1000, 50)
            expect(result).toBe(9950) // (110 - 100) * 1000 - 50
        })
    })
    
    describe('calculateDrawdown', () => {
        it('最大ドローダウンを計算する', () => {
            const equityCurve = [100000, 105000, 98000, 102000, 95000, 110000]
            const result = calculateDrawdown(equityCurve)
            expect(result).toBeCloseTo(0.095, 3) // 9.5%
        })
    })
    
    describe('formatCurrency', () => {
        it('USDを正しくフォーマットする', () => {
            const result = formatCurrency(1234.56, 'USD')
            expect(result).toBe('$1,234.56')
        })
        
        it('JPYを正しくフォーマットする', () => {
            const result = formatCurrency(123456, 'JPY')
            expect(result).toBe('¥123,456')
        })
    })
})
```

## 3. 統合テスト

### 3.1 API統合テスト
```go
// tests/integration/api_test.go
package integration

import (
    "testing"
    "net/http"
    "net/http/httptest"
    "encoding/json"
    "bytes"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestStrategyAPI(t *testing.T) {
    // テストサーバー起動
    app := setupTestApp()
    defer app.Cleanup()
    
    t.Run("戦略作成", func(t *testing.T) {
        // リクエスト準備
        strategyData := map[string]interface{}{
            "name": "Test Strategy",
            "description": "Test Description",
            "category": "trend_following",
            "script_content": "def on_bar(context, data): pass",
        }
        
        body, _ := json.Marshal(strategyData)
        req := httptest.NewRequest("POST", "/api/v1/strategies", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Bearer "+getTestToken())
        
        // リクエスト実行
        w := httptest.NewRecorder()
        app.ServeHTTP(w, req)
        
        // レスポンス検証
        assert.Equal(t, http.StatusCreated, w.Code)
        
        var response map[string]interface{}
        err := json.Unmarshal(w.Body.Bytes(), &response)
        require.NoError(t, err)
        
        assert.True(t, response["success"].(bool))
        assert.NotNil(t, response["data"])
    })
    
    t.Run("戦略一覧取得", func(t *testing.T) {
        req := httptest.NewRequest("GET", "/api/v1/strategies", nil)
        req.Header.Set("Authorization", "Bearer "+getTestToken())
        
        w := httptest.NewRecorder()
        app.ServeHTTP(w, req)
        
        assert.Equal(t, http.StatusOK, w.Code)
        
        var response map[string]interface{}
        err := json.Unmarshal(w.Body.Bytes(), &response)
        require.NoError(t, err)
        
        assert.True(t, response["success"].(bool))
        strategies := response["data"].(map[string]interface{})["strategies"].([]interface{})
        assert.GreaterOrEqual(t, len(strategies), 1)
    })
}

func TestBacktestAPI(t *testing.T) {
    app := setupTestApp()
    defer app.Cleanup()
    
    t.Run("バックテスト実行", func(t *testing.T) {
        backtestData := map[string]interface{}{
            "strategy_version_id": 1,
            "symbols": []string{"AAPL", "GOOGL"},
            "start_date": "2024-01-01",
            "end_date": "2024-01-31",
            "initial_capital": 100000,
        }
        
        body, _ := json.Marshal(backtestData)
        req := httptest.NewRequest("POST", "/api/v1/backtests", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Bearer "+getTestToken())
        
        w := httptest.NewRecorder()
        app.ServeHTTP(w, req)
        
        assert.Equal(t, http.StatusOK, w.Code)
        
        var response map[string]interface{}
        err := json.Unmarshal(w.Body.Bytes(), &response)
        require.NoError(t, err)
        
        assert.True(t, response["success"].(bool))
        assert.NotEmpty(t, response["data"].(map[string]interface{})["backtest_id"])
    })
}
```

### 3.2 データベース統合テスト
```go
// tests/integration/database_test.go
package integration

import (
    "testing"
    "context"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestStrategyRepository(t *testing.T) {
    db := setupTestDB()
    defer db.Cleanup()
    
    repo := NewStrategyRepository(db)
    
    t.Run("戦略作成と取得", func(t *testing.T) {
        // 戦略作成
        strategy := &Strategy{
            Name:        "Test Strategy",
            Description: "Test Description",
            Category:    "trend_following",
            IsActive:    true,
        }
        
        created, err := repo.Create(context.Background(), strategy)
        require.NoError(t, err)
        assert.NotZero(t, created.ID)
        
        // 戦略取得
        retrieved, err := repo.GetByID(context.Background(), created.ID)
        require.NoError(t, err)
        assert.Equal(t, strategy.Name, retrieved.Name)
        assert.Equal(t, strategy.Description, retrieved.Description)
    })
    
    t.Run("戦略一覧取得", func(t *testing.T) {
        strategies, err := repo.List(context.Background(), &StrategyFilter{
            Category: "trend_following",
            IsActive: true,
        })
        require.NoError(t, err)
        assert.GreaterOrEqual(t, len(strategies), 1)
    })
}

func TestOrderRepository(t *testing.T) {
    db := setupTestDB()
    defer db.Cleanup()
    
    repo := NewOrderRepository(db)
    
    t.Run("注文作成と状態更新", func(t *testing.T) {
        // 注文作成
        order := &Order{
            ClientOrderID:     "test_order_123",
            StrategyVersionID: 1,
            Symbol:           "AAPL",
            Side:             "buy",
            OrderType:        "market",
            Quantity:         100,
            Status:           "pending",
        }
        
        created, err := repo.Create(context.Background(), order)
        require.NoError(t, err)
        assert.NotZero(t, created.ID)
        
        // 状態更新
        err = repo.UpdateStatus(context.Background(), created.ID, "submitted")
        require.NoError(t, err)
        
        // 更新確認
        updated, err := repo.GetByID(context.Background(), created.ID)
        require.NoError(t, err)
        assert.Equal(t, "submitted", updated.Status)
    })
}
```

### 3.3 ブローカーアダプタ統合テスト
```go
// tests/integration/broker_test.go
package integration

import (
    "testing"
    "context"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestBrokerAdapter(t *testing.T) {
    adapter := setupTestBrokerAdapter()
    defer adapter.Cleanup()
    
    t.Run("接続テスト", func(t *testing.T) {
        err := adapter.Connect(context.Background())
        require.NoError(t, err)
        
        status := adapter.GetConnectionStatus()
        assert.Equal(t, "connected", status.Status)
    })
    
    t.Run("市場データ取得", func(t *testing.T) {
        data, err := adapter.GetMarketData(context.Background(), "AAPL", "1d", 10)
        require.NoError(t, err)
        assert.Len(t, data, 10)
        
        // データ形式検証
        for _, bar := range data {
            assert.NotZero(t, bar.Open)
            assert.NotZero(t, bar.High)
            assert.NotZero(t, bar.Low)
            assert.NotZero(t, bar.Close)
            assert.NotZero(t, bar.Volume)
        }
    })
    
    t.Run("注文発注", func(t *testing.T) {
        order := &Order{
            ClientOrderID: "test_order_456",
            Symbol:        "AAPL",
            Side:          "buy",
            OrderType:     "market",
            Quantity:      100,
        }
        
        result, err := adapter.PlaceOrder(context.Background(), order)
        require.NoError(t, err)
        assert.NotEmpty(t, result.BrokerOrderID)
        assert.Equal(t, "submitted", result.Status)
    })
}
```

## 4. E2Eテスト

### 4.1 Playwright E2Eテスト
```typescript
// apps/web/e2e/strategy-management.spec.ts
import { test, expect } from '@playwright/test'

test.describe('戦略管理', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login')
        await page.fill('[data-testid="email"]', 'test@example.com')
        await page.fill('[data-testid="password"]', 'password123')
        await page.fill('[data-testid="totp"]', '123456')
        await page.click('[data-testid="login-button"]')
        await page.waitForURL('/dashboard')
    })
    
    test('戦略作成フロー', async ({ page }) => {
        // 戦略センターに移動
        await page.click('[data-testid="nav-strategies"]')
        await page.click('[data-testid="create-strategy-button"]')
        
        // 戦略情報入力
        await page.fill('[data-testid="strategy-name"]', 'Test EMA Strategy')
        await page.fill('[data-testid="strategy-description"]', 'Test description')
        await page.selectOption('[data-testid="strategy-category"]', 'trend_following')
        
        // パラメータ設定
        await page.fill('[data-testid="fast-period"]', '9')
        await page.fill('[data-testid="slow-period"]', '21')
        await page.fill('[data-testid="risk-per-trade"]', '0.0025')
        
        // 保存
        await page.click('[data-testid="save-strategy"]')
        
        // 成功確認
        await expect(page.locator('[data-testid="success-message"]')).toBeVisible()
        await expect(page.locator('text=Test EMA Strategy')).toBeVisible()
    })
    
    test('バックテスト実行', async ({ page }) => {
        // 戦略選択
        await page.click('[data-testid="nav-strategies"]')
        await page.click('[data-testid="strategy-item-1"]')
        await page.click('[data-testid="run-backtest"]')
        
        // バックテスト設定
        await page.fill('[data-testid="start-date"]', '2024-01-01')
        await page.fill('[data-testid="end-date"]', '2024-01-31')
        await page.fill('[data-testid="initial-capital"]', '100000')
        await page.click('[data-testid="start-backtest"]')
        
        // 実行中表示確認
        await expect(page.locator('[data-testid="backtest-progress"]')).toBeVisible()
        
        // 完了待機
        await page.waitForSelector('[data-testid="backtest-complete"]', { timeout: 60000 })
        
        // 結果確認
        await expect(page.locator('[data-testid="total-return"]')).toBeVisible()
        await expect(page.locator('[data-testid="sharpe-ratio"]')).toBeVisible()
        await expect(page.locator('[data-testid="max-drawdown"]')).toBeVisible()
    })
    
    test('Paper Trading開始', async ({ page }) => {
        // バックテスト完了後、Paper Trading開始
        await page.click('[data-testid="start-paper-trading"]')
        
        // 確認ダイアログ
        await page.click('[data-testid="confirm-paper-trading"]')
        
        // 開始確認
        await expect(page.locator('[data-testid="paper-trading-active"]')).toBeVisible()
        await expect(page.locator('[data-testid="strategy-status"]')).toHaveText('Paper Trading')
    })
})

test.describe('モニタリング', () => {
    test('PnL表示', async ({ page }) => {
        await page.goto('/monitoring')
        
        // PnLチャート表示確認
        await expect(page.locator('[data-testid="pnl-chart"]')).toBeVisible()
        
        // 期間選択
        await page.click('[data-testid="period-selector"]')
        await page.click('[data-testid="period-1w"]')
        
        // チャート更新確認
        await expect(page.locator('[data-testid="pnl-chart"]')).toBeVisible()
    })
    
    test('注文履歴', async ({ page }) => {
        await page.goto('/orders')
        
        // 注文一覧表示確認
        await expect(page.locator('[data-testid="orders-table"]')).toBeVisible()
        
        // フィルター適用
        await page.selectOption('[data-testid="status-filter"]', 'filled')
        await page.fill('[data-testid="symbol-filter"]', 'AAPL')
        
        // フィルター結果確認
        const rows = page.locator('[data-testid="order-row"]')
        await expect(rows).toHaveCount(await rows.count())
    })
})
```

### 4.2 API E2Eテスト
```go
// tests/e2e/api_e2e_test.go
package e2e

import (
    "testing"
    "time"
    "net/http"
    "encoding/json"
    "bytes"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCompleteTradingFlow(t *testing.T) {
    client := setupE2EClient()
    
    t.Run("完全な取引フロー", func(t *testing.T) {
        // 1. 戦略作成
        strategyID := createTestStrategy(t, client)
        
        // 2. バックテスト実行
        backtestID := runBacktest(t, client, strategyID)
        
        // 3. バックテスト完了待機
        waitForBacktestCompletion(t, client, backtestID)
        
        // 4. Paper Trading開始
        paperTradingID := startPaperTrading(t, client, strategyID)
        
        // 5. 市場データ待機
        time.Sleep(5 * time.Second)
        
        // 6. 注文確認
        orders := getOrders(t, client, paperTradingID)
        assert.GreaterOrEqual(t, len(orders), 0)
        
        // 7. ポジション確認
        positions := getPositions(t, client, paperTradingID)
        assert.GreaterOrEqual(t, len(positions), 0)
        
        // 8. Paper Trading停止
        stopPaperTrading(t, client, paperTradingID)
    })
}

func createTestStrategy(t *testing.T, client *E2EClient) string {
    strategyData := map[string]interface{}{
        "name": "E2E Test Strategy",
        "description": "E2E test strategy",
        "category": "trend_following",
        "script_content": `
def on_bar(context, data):
    symbol = context.symbol
    current_price = get_current_price(symbol)
    
    # 簡単な買いシグナル
    if current_price > 100:
        place_order(symbol, "buy", "market", 100)
`,
    }
    
    resp := client.Post("/api/v1/strategies", strategyData)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    var result map[string]interface{}
    err := json.Unmarshal(resp.Body, &result)
    require.NoError(t, err)
    
    return result["data"].(map[string]interface{})["id"].(string)
}

func runBacktest(t *testing.T, client *E2EClient, strategyID string) string {
    backtestData := map[string]interface{}{
        "strategy_version_id": strategyID,
        "symbols": []string{"AAPL"},
        "start_date": "2024-01-01",
        "end_date": "2024-01-31",
        "initial_capital": 100000,
    }
    
    resp := client.Post("/api/v1/backtests", backtestData)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    var result map[string]interface{}
    err := json.Unmarshal(resp.Body, &result)
    require.NoError(t, err)
    
    return result["data"].(map[string]interface{})["backtest_id"].(string)
}

func waitForBacktestCompletion(t *testing.T, client *E2EClient, backtestID string) {
    for i := 0; i < 60; i++ { // 最大60秒待機
        resp := client.Get("/api/v1/backtests/" + backtestID)
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        
        var result map[string]interface{}
        err := json.Unmarshal(resp.Body, &result)
        require.NoError(t, err)
        
        status := result["data"].(map[string]interface{})["status"].(string)
        if status == "completed" {
            return
        }
        
        time.Sleep(1 * time.Second)
    }
    
    t.Fatal("Backtest did not complete within timeout")
}
```

## 5. パフォーマンステスト

### 5.1 負荷テスト
```go
// tests/performance/load_test.go
package performance

import (
    "testing"
    "net/http"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/loadimpact/k6/lib"
)

func TestAPILoad(t *testing.T) {
    t.Run("戦略一覧取得負荷テスト", func(t *testing.T) {
        // 100 RPSで30秒間テスト
        results := runLoadTest(t, LoadTestConfig{
            Endpoint:    "/api/v1/strategies",
            Method:      "GET",
            RPS:         100,
            Duration:    30 * time.Second,
            Concurrent:  10,
        })
        
        assert.Less(t, results.AverageResponseTime, 100*time.Millisecond)
        assert.Less(t, results.ErrorRate, 0.01) // 1%以下
        assert.Greater(t, results.Throughput, 95.0) // 95 RPS以上
    })
    
    t.Run("バックテスト負荷テスト", func(t *testing.T) {
        // 10 RPSで60秒間テスト
        results := runLoadTest(t, LoadTestConfig{
            Endpoint:    "/api/v1/backtests",
            Method:      "POST",
            RPS:         10,
            Duration:    60 * time.Second,
            Concurrent:  5,
            Payload:     getBacktestPayload(),
        })
        
        assert.Less(t, results.AverageResponseTime, 500*time.Millisecond)
        assert.Less(t, results.ErrorRate, 0.05) // 5%以下
    })
}

func TestDatabaseLoad(t *testing.T) {
    t.Run("大量データ読み取り", func(t *testing.T) {
        // 1000件の戦略データでテスト
        results := runDatabaseLoadTest(t, DatabaseLoadTestConfig{
            Query:       "SELECT * FROM strategies WHERE is_active = true",
            Concurrent:  20,
            Duration:    30 * time.Second,
        })
        
        assert.Less(t, results.AverageQueryTime, 50*time.Millisecond)
        assert.Less(t, results.ErrorRate, 0.001) // 0.1%以下
    })
}
```

### 5.2 ストレステスト
```go
// tests/performance/stress_test.go
package performance

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestSystemStress(t *testing.T) {
    t.Run("高負荷ストレステスト", func(t *testing.T) {
        // 段階的に負荷を上げる
        stressLevels := []int{50, 100, 200, 500, 1000}
        
        for _, rps := range stressLevels {
            t.Logf("Testing at %d RPS", rps)
            
            results := runStressTest(t, StressTestConfig{
                Endpoint:    "/api/v1/strategies",
                RPS:         rps,
                Duration:    60 * time.Second,
                RampUp:      10 * time.Second,
            })
            
            // システムが応答し続けることを確認
            assert.Less(t, results.ErrorRate, 0.1) // 10%以下
            assert.Greater(t, results.Throughput, float64(rps)*0.8) // 80%以上のスループット
        }
    })
    
    t.Run("メモリリークテスト", func(t *testing.T) {
        // 長時間実行してメモリ使用量を監視
        initialMemory := getMemoryUsage()
        
        // 1時間の軽い負荷
        runStressTest(t, StressTestConfig{
            Endpoint:    "/api/v1/strategies",
            RPS:         10,
            Duration:    1 * time.Hour,
        })
        
        finalMemory := getMemoryUsage()
        memoryIncrease := (finalMemory - initialMemory) / initialMemory
        
        // メモリ増加が50%以下
        assert.Less(t, memoryIncrease, 0.5)
    })
}
```

## 6. セキュリティテスト

### 6.1 認証・認可テスト
```go
// tests/security/auth_test.go
package security

import (
    "testing"
    "net/http"
    "github.com/stretchr/testify/assert"
)

func TestAuthentication(t *testing.T) {
    t.Run("無効なトークン", func(t *testing.T) {
        resp := makeRequest(t, "/api/v1/strategies", "GET", nil, "invalid-token")
        assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
    })
    
    t.Run("期限切れトークン", func(t *testing.T) {
        expiredToken := generateExpiredToken()
        resp := makeRequest(t, "/api/v1/strategies", "GET", nil, expiredToken)
        assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
    })
    
    t.Run("不正なJWT形式", func(t *testing.T) {
        resp := makeRequest(t, "/api/v1/strategies", "GET", nil, "not-a-jwt-token")
        assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
    })
}

func TestAuthorization(t *testing.T) {
    t.Run("他のユーザーの戦略にアクセス", func(t *testing.T) {
        otherUserToken := generateTokenForUser("other-user")
        resp := makeRequest(t, "/api/v1/strategies/999", "GET", nil, otherUserToken)
        assert.Equal(t, http.StatusForbidden, resp.StatusCode)
    })
    
    t.Run("管理者権限が必要なエンドポイント", func(t *testing.T) {
        userToken := generateTokenForUser("regular-user")
        resp := makeRequest(t, "/api/v1/admin/users", "GET", nil, userToken)
        assert.Equal(t, http.StatusForbidden, resp.StatusCode)
    })
}
```

### 6.2 入力検証テスト
```go
// tests/security/validation_test.go
package security

import (
    "testing"
    "net/http"
    "github.com/stretchr/testify/assert"
)

func TestInputValidation(t *testing.T) {
    t.Run("SQLインジェクション対策", func(t *testing.T) {
        maliciousInput := "'; DROP TABLE strategies; --"
        payload := map[string]interface{}{
            "name": maliciousInput,
        }
        
        resp := makeRequest(t, "/api/v1/strategies", "POST", payload, validToken)
        // バリデーションエラーになることを確認
        assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
    })
    
    t.Run("XSS対策", func(t *testing.T) {
        xssPayload := "<script>alert('xss')</script>"
        payload := map[string]interface{}{
            "description": xssPayload,
        }
        
        resp := makeRequest(t, "/api/v1/strategies", "POST", payload, validToken)
        // エスケープされていることを確認
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        // 取得時にエスケープされていることを確認
        getResp := makeRequest(t, "/api/v1/strategies/1", "GET", nil, validToken)
        assert.NotContains(t, string(getResp.Body), "<script>")
    })
    
    t.Run("CSRF対策", func(t *testing.T) {
        // CSRFトークンなしでリクエスト
        resp := makeRequestWithoutCSRF(t, "/api/v1/strategies", "POST", nil, validToken)
        assert.Equal(t, http.StatusForbidden, resp.StatusCode)
    })
}
```

## 7. テスト実行設定

### 7.1 Makefile
```makefile
# Makefile
.PHONY: test test-unit test-integration test-e2e test-performance

# 全テスト実行
test: test-unit test-integration test-e2e

# 単体テスト
test-unit:
	@echo "Running unit tests..."
	cd apps/api && go test -v -race -coverprofile=coverage.out ./...
	cd apps/web && npm run test:unit

# 統合テスト
test-integration:
	@echo "Running integration tests..."
	cd tests/integration && go test -v -tags=integration

# E2Eテスト
test-e2e:
	@echo "Running E2E tests..."
	cd apps/web && npm run test:e2e

# パフォーマンステスト
test-performance:
	@echo "Running performance tests..."
	cd tests/performance && go test -v -tags=performance

# カバレッジレポート
coverage:
	@echo "Generating coverage report..."
	cd apps/api && go tool cover -html=coverage.out -o coverage.html
	open coverage.html

# テストデータ準備
test-data:
	@echo "Preparing test data..."
	cd tests && go run setup_test_data.go

# テスト環境クリーンアップ
test-cleanup:
	@echo "Cleaning up test environment..."
	docker-compose -f docker-compose.test.yml down -v
```

### 7.2 GitHub Actions
```yaml
# .github/workflows/test.yml
name: Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run Go unit tests
      run: |
        cd apps/api
        go test -v -race -coverprofile=coverage.out ./...
        go tool cover -func=coverage.out
    
    - name: Set up Node.js
      uses: actions/setup-node@v3
      with:
        node-version: '18'
        cache: 'npm'
    
    - name: Install dependencies
      run: cd apps/web && npm ci
    
    - name: Run frontend unit tests
      run: cd apps/web && npm run test:unit

  integration-tests:
    runs-on: ubuntu-latest
    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: test
          MYSQL_DATABASE: trading_system_test
        options: >-
          --health-cmd "mysqladmin ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run integration tests
      run: |
        cd tests/integration
        go test -v -tags=integration

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Node.js
      uses: actions/setup-node@v3
      with:
        node-version: '18'
        cache: 'npm'
    
    - name: Install dependencies
      run: cd apps/web && npm ci
    
    - name: Install Playwright
      run: cd apps/web && npx playwright install --with-deps
    
    - name: Start test environment
      run: |
        docker-compose -f docker-compose.test.yml up -d
        sleep 30
    
    - name: Run E2E tests
      run: cd apps/web && npm run test:e2e
    
    - name: Upload test results
      uses: actions/upload-artifact@v3
      if: failure()
      with:
        name: playwright-report
        path: apps/web/playwright-report/
        retention-days: 30

  performance-tests:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Start test environment
      run: |
        docker-compose -f docker-compose.test.yml up -d
        sleep 30
    
    - name: Run performance tests
      run: |
        cd tests/performance
        go test -v -tags=performance
```

## 8. テストデータ管理

### 8.1 テストデータファクトリ
```go
// tests/factories/strategy_factory.go
package factories

import (
    "time"
    "github.com/trading-system/internal/strategy"
)

type StrategyFactory struct{}

func (f *StrategyFactory) Create(params map[string]interface{}) *strategy.Strategy {
    defaults := map[string]interface{}{
        "name":        "Test Strategy",
        "description": "Test Description",
        "category":    "trend_following",
        "is_active":   true,
        "created_at":  time.Now(),
    }
    
    // デフォルト値をマージ
    for key, value := range defaults {
        if _, exists := params[key]; !exists {
            params[key] = value
        }
    }
    
    return &strategy.Strategy{
        Name:        params["name"].(string),
        Description: params["description"].(string),
        Category:    params["category"].(string),
        IsActive:    params["is_active"].(bool),
        CreatedAt:   params["created_at"].(time.Time),
    }
}

func (f *StrategyFactory) CreateMany(count int, params map[string]interface{}) []*strategy.Strategy {
    strategies := make([]*strategy.Strategy, count)
    for i := 0; i < count; i++ {
        strategies[i] = f.Create(params)
    }
    return strategies
}
```

### 8.2 テストデータセットアップ
```go
// tests/setup_test_data.go
package main

import (
    "database/sql"
    "log"
    "os"
    "github.com/trading-system/tests/factories"
)

func main() {
    db, err := sql.Open("mysql", os.Getenv("TEST_DATABASE_URL"))
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // テストデータ作成
    strategyFactory := &factories.StrategyFactory{}
    
    // 基本戦略データ
    strategies := strategyFactory.CreateMany(10, map[string]interface{}{
        "category": "trend_following",
    })
    
    // データベースに挿入
    for _, strategy := range strategies {
        // データベース挿入処理
    }
    
    log.Println("Test data setup completed")
}
```