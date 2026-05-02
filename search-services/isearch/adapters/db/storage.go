package db

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"yadro.com/course/isearch/core"
)

type DBComicsDto struct {
	ID    int            `db:"id"`
	URL   string         `db:"url"`
	Words pq.StringArray `db:"words"`
}
type DBLightComicDto struct {
	ID  int    `db:"id"`
	URL string `db:"url"`
}
type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {
	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(50)
	// db.SetConnMaxLifetime(5 * time.Minute)

	return &DB{
		log:  log,
		conn: db,
	}, nil
}

func (db *DB) GetAllComics(ctx context.Context) ([]core.DBComic, error) {
	dbRes := []DBComicsDto{}

	query := `SELECT id, url, words FROM comics`

	err := db.conn.SelectContext(ctx, &dbRes, query)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	res := make([]core.DBComic, len(dbRes))

	for k, v := range dbRes {
		res[k] = core.DBComic{
			ID:    v.ID,
			URL:   v.URL,
			Words: v.Words,
		}
	}

	return res, nil
}

func (db *DB) GetComicsByIDs(ctx context.Context, ids []int) ([]core.DBLightComic, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	dbRes := []DBLightComicDto{}

	query, args, err := sqlx.In("SELECT id, url FROM comics WHERE id IN (?)", ids)
	if err != nil {
		return nil, fmt.Errorf("sqlx: %w", err)
	}

	query = db.conn.Rebind(query)

	err = db.conn.SelectContext(ctx, &dbRes, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	res := make([]core.DBLightComic, len(dbRes))

	for k, v := range dbRes {
		res[k] = core.DBLightComic{
			ID:  v.ID,
			URL: v.URL,
		}
	}

	return res, nil
}
