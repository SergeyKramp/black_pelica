// Package db provides PostgreSQL connection setup.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"hema/ces/internal/config"
)

// Connect creates a pgx connection pool using the given database config and verifies
// connectivity with a ping.
// Pool size is controlled by cfg.Pool.MaxConns and cfg.Pool.MinConns.
// The caller is responsible for closing the pool.
func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	poolCfg.MaxConns = cfg.Pool.MaxConns
	poolCfg.MinConns = cfg.Pool.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
