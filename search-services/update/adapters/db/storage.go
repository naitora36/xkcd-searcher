package db

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"yadro.com/course/update/core"
)

type DB struct {
	log  *slog.Logger
	Conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {
	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	return &DB{
		log:  log,
		Conn: db,
	}, nil
}

func (db *DB) Add(ctx context.Context, comics core.Comics) error {
	query := `
		INSERT INTO comics (id, url, words)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING`

	_, err := db.Conn.Exec(query, comics.ID, comics.URL, comics.Words)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) Stats(ctx context.Context) (core.DBStats, error) {
	var stats core.DBStats

	countQuery := `
        SELECT 
            COUNT(*), 
            COALESCE(SUM(cardinality(words)), 0) 
        FROM comics`

	err := db.Conn.QueryRowxContext(ctx, countQuery).Scan(&stats.ComicsFetched, &stats.WordsTotal)
	if err != nil {
		return stats, err
	}

	uniqueQuery := `
        SELECT COUNT(DISTINCT word) 
        FROM (
            SELECT unnest(words) as word
            FROM comics
        ) AS t`

	err = db.Conn.GetContext(ctx, &stats.WordsUnique, uniqueQuery)

	return stats, err
}

func (db *DB) IDs(ctx context.Context) ([]int, error) {
	var ids []int
	query := `SELECT id FROM comics`
	err := db.Conn.SelectContext(ctx, &ids, query)
	return ids, err
}

func (db *DB) Drop(ctx context.Context) error {
	query := `TRUNCATE TABLE comics`

	_, err := db.Conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	return nil
}
