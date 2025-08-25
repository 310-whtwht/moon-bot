package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/moomoo-trading/api/internal/broker"
)

// TaxExporter handles tax-related exports
type TaxExporter struct {
	// Add any dependencies here
}

// NewTaxExporter creates a new tax exporter
func NewTaxExporter() *TaxExporter {
	return &TaxExporter{}
}

// ExportTWSFormat exports trades in TWS (Interactive Brokers) format
func (te *TaxExporter) ExportTWSFormat(ctx context.Context, trades []broker.Trade, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Date/Time",
		"Symbol",
		"Description",
		"Action",
		"Quantity",
		"Price",
		"Commission",
		"Proceeds",
		"Cost Basis",
		"Realized P&L",
		"Currency",
		"Trade ID",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write trade data
	for _, trade := range trades {
		action := "BUY"
		if trade.Side == broker.OrderSideSell {
			action = "SELL"
		}

		proceeds := trade.Quantity * trade.Price
		costBasis := proceeds
		realizedPnL := 0.0

		if action == "SELL" {
			// For sells, calculate realized P&L
			// This is a simplified calculation - in practice, you'd need to track cost basis
			realizedPnL = proceeds - costBasis - trade.Commission
		}

		row := []string{
			trade.TradeTime.Format("2006-01-02 15:04:05"),
			trade.Symbol,
			trade.Symbol, // Description same as symbol for simplicity
			action,
			fmt.Sprintf("%.6f", trade.Quantity),
			fmt.Sprintf("%.2f", trade.Price),
			fmt.Sprintf("%.2f", trade.Commission),
			fmt.Sprintf("%.2f", proceeds),
			fmt.Sprintf("%.2f", costBasis),
			fmt.Sprintf("%.2f", realizedPnL),
			"USD", // Default currency
			trade.ID,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// ExportJapaneseFormat exports trades in Japanese tax format
func (te *TaxExporter) ExportJapaneseFormat(ctx context.Context, trades []broker.Trade, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header in Japanese
	header := []string{
		"取引日時",
		"銘柄コード",
		"銘柄名",
		"取引区分",
		"数量",
		"単価",
		"手数料",
		"取引金額",
		"取得価額",
		"譲渡損益",
		"通貨",
		"取引ID",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write trade data
	for _, trade := range trades {
		action := "買付"
		if trade.Side == broker.OrderSideSell {
			action = "売却"
		}

		proceeds := trade.Quantity * trade.Price
		costBasis := proceeds
		realizedPnL := 0.0

		if action == "売却" {
			// For sells, calculate realized P&L
			realizedPnL = proceeds - costBasis - trade.Commission
		}

		row := []string{
			trade.TradeTime.Format("2006-01-02 15:04:05"),
			trade.Symbol,
			trade.Symbol, // Symbol name same as code for simplicity
			action,
			fmt.Sprintf("%.6f", trade.Quantity),
			fmt.Sprintf("%.2f", trade.Price),
			fmt.Sprintf("%.2f", trade.Commission),
			fmt.Sprintf("%.2f", proceeds),
			fmt.Sprintf("%.2f", costBasis),
			fmt.Sprintf("%.2f", realizedPnL),
			"USD", // Default currency
			trade.ID,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// ExportAnnualReport exports annual trading report
func (te *TaxExporter) ExportAnnualReport(ctx context.Context, trades []broker.Trade, year int, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Filter trades for the specified year
	var yearTrades []broker.Trade
	for _, trade := range trades {
		if trade.TradeTime.Year() == year {
			yearTrades = append(yearTrades, trade)
		}
	}

	// Calculate summary statistics
	summary := te.calculateAnnualSummary(yearTrades)

	// Write summary
	writer.Write([]string{"Annual Trading Summary", fmt.Sprintf("%d", year)})
	writer.Write([]string{})
	writer.Write([]string{"Total Trades", fmt.Sprintf("%d", len(yearTrades))})
	writer.Write([]string{"Total Volume", fmt.Sprintf("%.2f", summary.TotalVolume)})
	writer.Write([]string{"Total Commission", fmt.Sprintf("%.2f", summary.TotalCommission)})
	writer.Write([]string{"Total Proceeds", fmt.Sprintf("%.2f", summary.TotalProceeds)})
	writer.Write([]string{"Total Cost Basis", fmt.Sprintf("%.2f", summary.TotalCostBasis)})
	writer.Write([]string{"Total Realized P&L", fmt.Sprintf("%.2f", summary.TotalRealizedPnL)})
	writer.Write([]string{})

	// Write detailed trades
	writer.Write([]string{"Detailed Trades"})
	writer.Write([]string{
		"Date/Time",
		"Symbol",
		"Action",
		"Quantity",
		"Price",
		"Commission",
		"Proceeds",
		"Cost Basis",
		"Realized P&L",
	})

	for _, trade := range yearTrades {
		action := "BUY"
		if trade.Side == broker.OrderSideSell {
			action = "SELL"
		}

		proceeds := trade.Quantity * trade.Price
		costBasis := proceeds
		realizedPnL := 0.0

		if action == "SELL" {
			realizedPnL = proceeds - costBasis - trade.Commission
		}

		writer.Write([]string{
			trade.TradeTime.Format("2006-01-02 15:04:05"),
			trade.Symbol,
			action,
			fmt.Sprintf("%.6f", trade.Quantity),
			fmt.Sprintf("%.2f", trade.Price),
			fmt.Sprintf("%.2f", trade.Commission),
			fmt.Sprintf("%.2f", proceeds),
			fmt.Sprintf("%.2f", costBasis),
			fmt.Sprintf("%.2f", realizedPnL),
		})
	}

	return nil
}

// AnnualSummary contains annual trading summary
type AnnualSummary struct {
	TotalTrades       int
	TotalVolume       float64
	TotalCommission   float64
	TotalProceeds     float64
	TotalCostBasis    float64
	TotalRealizedPnL  float64
}

// calculateAnnualSummary calculates annual trading summary
func (te *TaxExporter) calculateAnnualSummary(trades []broker.Trade) *AnnualSummary {
	summary := &AnnualSummary{
		TotalTrades: len(trades),
	}

	for _, trade := range trades {
		summary.TotalVolume += trade.Quantity
		summary.TotalCommission += trade.Commission
		
		proceeds := trade.Quantity * trade.Price
		summary.TotalProceeds += proceeds
		
		costBasis := proceeds
		summary.TotalCostBasis += costBasis

		if trade.Side == broker.OrderSideSell {
			realizedPnL := proceeds - costBasis - trade.Commission
			summary.TotalRealizedPnL += realizedPnL
		}
	}

	return summary
}

// ExportForm8949 exports Form 8949 format (US tax form)
func (te *TaxExporter) ExportForm8949(ctx context.Context, trades []broker.Trade, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Form 8949 header
	header := []string{
		"Form 8949 - Sales and Other Dispositions of Capital Assets",
		"Part I - Short-term transactions",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Column headers
	columnHeader := []string{
		"1a", "1b", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11",
	}

	if err := writer.Write(columnHeader); err != nil {
		return fmt.Errorf("failed to write column header: %w", err)
	}

	// Write trade data
	for _, trade := range trades {
		if trade.Side == broker.OrderSideSell {
			// Only include sell transactions
			proceeds := trade.Quantity * trade.Price
			costBasis := proceeds // Simplified
			gainLoss := proceeds - costBasis - trade.Commission

			row := []string{
				"", // 1a - Description of property
				"", // 1b - Date acquired
				"", // 2 - Date sold
				"", // 3 - Proceeds
				"", // 4 - Cost or other basis
				"", // 5 - Code from instructions
				"", // 6 - Amount of adjustment
				"", // 7 - Gain or loss
				"", // 8 - Unrealized gain or loss
				"", // 9 - Basis adjustment
				"", // 10 - Gain or loss
				"", // 11 - Unrealized gain or loss
			}

			if err := writer.Write(row); err != nil {
				return fmt.Errorf("failed to write row: %w", err)
			}
		}
	}

	return nil
}