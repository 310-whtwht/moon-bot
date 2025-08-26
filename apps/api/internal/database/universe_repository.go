package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// UniverseRepository handles database operations for universe symbols
type UniverseRepository struct {
	db *sql.DB
}

// NewUniverseRepository creates a new universe repository
func NewUniverseRepository(db *sql.DB) *UniverseRepository {
	return &UniverseRepository{db: db}
}

// CreateSymbol creates a new universe symbol
func (r *UniverseRepository) CreateSymbol(ctx context.Context, symbol *UniverseSymbol) error {
	symbol.ID = uuid.New().String()
	symbol.CreatedAt = time.Now()
	symbol.UpdatedAt = time.Now()

	query := `
		INSERT INTO universe_symbols (id, symbol, name, exchange, asset_type, is_active, data_source, last_updated, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		symbol.ID, symbol.Symbol, symbol.Name, symbol.Exchange, symbol.AssetType,
		symbol.IsActive, symbol.DataSource, symbol.LastUpdated, symbol.CreatedAt, symbol.UpdatedAt)
	return err
}

// GetSymbolByID retrieves a universe symbol by ID
func (r *UniverseRepository) GetSymbolByID(ctx context.Context, id string) (*UniverseSymbol, error) {
	query := `
		SELECT id, symbol, name, exchange, asset_type, is_active, data_source, last_updated, created_at, updated_at
		FROM universe_symbols WHERE id = ?
	`

	var symbol UniverseSymbol
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&symbol.ID, &symbol.Symbol, &symbol.Name, &symbol.Exchange, &symbol.AssetType,
		&symbol.IsActive, &symbol.DataSource, &symbol.LastUpdated, &symbol.CreatedAt, &symbol.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &symbol, nil
}

// GetSymbolBySymbol retrieves a universe symbol by symbol name
func (r *UniverseRepository) GetSymbolBySymbol(ctx context.Context, symbolName string) (*UniverseSymbol, error) {
	query := `
		SELECT id, symbol, name, exchange, asset_type, is_active, data_source, last_updated, created_at, updated_at
		FROM universe_symbols WHERE symbol = ?
	`

	var symbol UniverseSymbol
	err := r.db.QueryRowContext(ctx, query, symbolName).Scan(
		&symbol.ID, &symbol.Symbol, &symbol.Name, &symbol.Exchange, &symbol.AssetType,
		&symbol.IsActive, &symbol.DataSource, &symbol.LastUpdated, &symbol.CreatedAt, &symbol.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &symbol, nil
}

// ListSymbols retrieves universe symbols with filtering
func (r *UniverseRepository) ListSymbols(ctx context.Context, exchange, assetType *string, isActive *bool, limit, offset int) ([]*UniverseSymbol, error) {
	query := `
		SELECT id, symbol, name, exchange, asset_type, is_active, data_source, last_updated, created_at, updated_at
		FROM universe_symbols WHERE 1=1
	`
	var args []interface{}

	if exchange != nil {
		query += " AND exchange = ?"
		args = append(args, *exchange)
	}
	if assetType != nil {
		query += " AND asset_type = ?"
		args = append(args, *assetType)
	}
	if isActive != nil {
		query += " AND is_active = ?"
		args = append(args, *isActive)
	}

	query += " ORDER BY symbol ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*UniverseSymbol
	for rows.Next() {
		var symbol UniverseSymbol
		err := rows.Scan(
			&symbol.ID, &symbol.Symbol, &symbol.Name, &symbol.Exchange, &symbol.AssetType,
			&symbol.IsActive, &symbol.DataSource, &symbol.LastUpdated, &symbol.CreatedAt, &symbol.UpdatedAt)
		if err != nil {
			return nil, err
		}
		symbols = append(symbols, &symbol)
	}

	return symbols, nil
}

// UpdateSymbol updates a universe symbol
func (r *UniverseRepository) UpdateSymbol(ctx context.Context, symbol *UniverseSymbol) error {
	symbol.UpdatedAt = time.Now()

	query := `
		UPDATE universe_symbols
		SET name = ?, exchange = ?, asset_type = ?, is_active = ?, data_source = ?, last_updated = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		symbol.Name, symbol.Exchange, symbol.AssetType, symbol.IsActive,
		symbol.DataSource, symbol.LastUpdated, symbol.UpdatedAt, symbol.ID)
	return err
}

// DeleteSymbol deletes a universe symbol
func (r *UniverseRepository) DeleteSymbol(ctx context.Context, id string) error {
	query := `DELETE FROM universe_symbols WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteSymbolBySymbol deletes a universe symbol by symbol name
func (r *UniverseRepository) DeleteSymbolBySymbol(ctx context.Context, symbolName string) error {
	query := `DELETE FROM universe_symbols WHERE symbol = ?`
	_, err := r.db.ExecContext(ctx, query, symbolName)
	return err
}

// BulkUpsertSymbols upserts multiple symbols
func (r *UniverseRepository) BulkUpsertSymbols(ctx context.Context, symbols []*UniverseSymbol) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO universe_symbols (id, symbol, name, exchange, asset_type, is_active, data_source, last_updated, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		name = VALUES(name),
		exchange = VALUES(exchange),
		asset_type = VALUES(asset_type),
		is_active = VALUES(is_active),
		data_source = VALUES(data_source),
		last_updated = VALUES(last_updated),
		updated_at = VALUES(updated_at)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, symbol := range symbols {
		if symbol.ID == "" {
			symbol.ID = uuid.New().String()
		}
		if symbol.CreatedAt.IsZero() {
			symbol.CreatedAt = time.Now()
		}
		symbol.UpdatedAt = time.Now()

		_, err := stmt.ExecContext(ctx,
			symbol.ID, symbol.Symbol, symbol.Name, symbol.Exchange, symbol.AssetType,
			symbol.IsActive, symbol.DataSource, symbol.LastUpdated, symbol.CreatedAt, symbol.UpdatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}