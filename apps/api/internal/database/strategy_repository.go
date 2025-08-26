package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StrategyRepository handles database operations for strategies
type StrategyRepository struct {
	db *sql.DB
}

// NewStrategyRepository creates a new strategy repository
func NewStrategyRepository(db *sql.DB) *StrategyRepository {
	return &StrategyRepository{db: db}
}

// CreatePackage creates a new strategy package
func (r *StrategyRepository) CreatePackage(ctx context.Context, pkg *StrategyPackage) error {
	pkg.ID = uuid.New().String()
	pkg.CreatedAt = time.Now()
	pkg.UpdatedAt = time.Now()

	query := `
		INSERT INTO strategy_packages (id, name, description, author, is_public, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		pkg.ID, pkg.Name, pkg.Description, pkg.Author, pkg.IsPublic, pkg.CreatedAt, pkg.UpdatedAt)
	return err
}

// GetPackageByID retrieves a strategy package by ID
func (r *StrategyRepository) GetPackageByID(ctx context.Context, id string) (*StrategyPackage, error) {
	query := `
		SELECT id, name, description, author, is_public, created_at, updated_at
		FROM strategy_packages WHERE id = ?
	`

	var pkg StrategyPackage
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&pkg.ID, &pkg.Name, &pkg.Description, &pkg.Author, &pkg.IsPublic, &pkg.CreatedAt, &pkg.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

// ListPackages retrieves all strategy packages
func (r *StrategyRepository) ListPackages(ctx context.Context, limit, offset int) ([]*StrategyPackage, error) {
	query := `
		SELECT id, name, description, author, is_public, created_at, updated_at
		FROM strategy_packages
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []*StrategyPackage
	for rows.Next() {
		var pkg StrategyPackage
		err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Description, &pkg.Author, &pkg.IsPublic, &pkg.CreatedAt, &pkg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		packages = append(packages, &pkg)
	}

	return packages, nil
}

// UpdatePackage updates a strategy package
func (r *StrategyRepository) UpdatePackage(ctx context.Context, pkg *StrategyPackage) error {
	pkg.UpdatedAt = time.Now()

	query := `
		UPDATE strategy_packages
		SET name = ?, description = ?, author = ?, is_public = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		pkg.Name, pkg.Description, pkg.Author, pkg.IsPublic, pkg.UpdatedAt, pkg.ID)
	return err
}

// DeletePackage deletes a strategy package
func (r *StrategyRepository) DeletePackage(ctx context.Context, id string) error {
	query := `DELETE FROM strategy_packages WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// CreateVersion creates a new strategy version
func (r *StrategyRepository) CreateVersion(ctx context.Context, version *StrategyVersion) error {
	version.ID = uuid.New().String()
	version.CreatedAt = time.Now()
	version.UpdatedAt = time.Now()

	query := `
		INSERT INTO strategy_versions (id, package_id, version, code, description, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		version.ID, version.PackageID, version.Version, version.Code, version.Description, version.IsActive, version.CreatedAt, version.UpdatedAt)
	return err
}

// GetVersionByID retrieves a strategy version by ID
func (r *StrategyRepository) GetVersionByID(ctx context.Context, id string) (*StrategyVersion, error) {
	query := `
		SELECT id, package_id, version, code, description, is_active, created_at, updated_at
		FROM strategy_versions WHERE id = ?
	`

	var version StrategyVersion
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&version.ID, &version.PackageID, &version.Version, &version.Code, &version.Description, &version.IsActive, &version.CreatedAt, &version.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

// GetActiveVersionByPackageID retrieves the active version of a package
func (r *StrategyRepository) GetActiveVersionByPackageID(ctx context.Context, packageID string) (*StrategyVersion, error) {
	query := `
		SELECT id, package_id, version, code, description, is_active, created_at, updated_at
		FROM strategy_versions WHERE package_id = ? AND is_active = true
	`

	var version StrategyVersion
	err := r.db.QueryRowContext(ctx, query, packageID).Scan(
		&version.ID, &version.PackageID, &version.Version, &version.Code, &version.Description, &version.IsActive, &version.CreatedAt, &version.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

// ListVersionsByPackageID retrieves all versions of a package
func (r *StrategyRepository) ListVersionsByPackageID(ctx context.Context, packageID string) ([]*StrategyVersion, error) {
	query := `
		SELECT id, package_id, version, code, description, is_active, created_at, updated_at
		FROM strategy_versions WHERE package_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*StrategyVersion
	for rows.Next() {
		var version StrategyVersion
		err := rows.Scan(&version.ID, &version.PackageID, &version.Version, &version.Code, &version.Description, &version.IsActive, &version.CreatedAt, &version.UpdatedAt)
		if err != nil {
			return nil, err
		}
		versions = append(versions, &version)
	}

	return versions, nil
}

// UpdateVersion updates a strategy version
func (r *StrategyRepository) UpdateVersion(ctx context.Context, version *StrategyVersion) error {
	version.UpdatedAt = time.Now()

	query := `
		UPDATE strategy_versions
		SET version = ?, code = ?, description = ?, is_active = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		version.Version, version.Code, version.Description, version.IsActive, version.UpdatedAt, version.ID)
	return err
}

// DeleteVersion deletes a strategy version
func (r *StrategyRepository) DeleteVersion(ctx context.Context, id string) error {
	query := `DELETE FROM strategy_versions WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// CreateParam creates a new strategy parameter
func (r *StrategyRepository) CreateParam(ctx context.Context, param *StrategyParam) error {
	param.ID = uuid.New().String()
	param.CreatedAt = time.Now()
	param.UpdatedAt = time.Now()

	query := `
		INSERT INTO strategy_params (id, version_id, param_name, param_type, default_value, description, is_required, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		param.ID, param.VersionID, param.ParamName, param.ParamType, param.DefaultValue, param.Description, param.IsRequired, param.CreatedAt, param.UpdatedAt)
	return err
}

// GetParamsByVersionID retrieves all parameters for a version
func (r *StrategyRepository) GetParamsByVersionID(ctx context.Context, versionID string) ([]*StrategyParam, error) {
	query := `
		SELECT id, version_id, param_name, param_type, default_value, description, is_required, created_at, updated_at
		FROM strategy_params WHERE version_id = ?
		ORDER BY param_name
	`

	rows, err := r.db.QueryContext(ctx, query, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var params []*StrategyParam
	for rows.Next() {
		var param StrategyParam
		err := rows.Scan(&param.ID, &param.VersionID, &param.ParamName, &param.ParamType, &param.DefaultValue, &param.Description, &param.IsRequired, &param.CreatedAt, &param.UpdatedAt)
		if err != nil {
			return nil, err
		}
		params = append(params, &param)
	}

	return params, nil
}