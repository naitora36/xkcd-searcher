package db

import (
	"context"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"yadro.com/course/search/core"
)

type DBComicsDto struct {
	ID    int            `db:"id"`
	URL   string         `db:"url"`
	Words pq.StringArray `db:"words"`
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
		slog.Warn("error id db layer", "error", err)
		return nil, err
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
