package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

type DB struct {
	*pgxpool.Pool
	Queries *queries.Queries
}

func NewPostgresDB(cfg *config.Config) (*DB, error) {
	// configure connection pool
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.DatabaseMaxConnections)
	poolConfig.MaxConnIdleTime = cfg.DatabaseMaxIdleTime
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.HealthCheckPeriod = time.Minute * 5

	// create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// ping database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		Pool:    pool,
		Queries: queries.New(pool),
	}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

func GetTenantSchema(tenantSlug string) string {
	return fmt.Sprintf("tenant_%s", tenantSlug)
}

func (db *DB) WithTenantSchema(ctx context.Context, tenantSlug string, fn func() error) error {
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	schema := GetTenantSchema(tenantSlug)

	// set search_path to tenant schema
	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		return fmt.Errorf("failed to set search_path: %w", err)
	}
	return fn()
}
