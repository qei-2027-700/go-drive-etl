package repository

import (
	"context"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New は STATE_BACKEND に応じた FileRepo と、その後始末をする関数を返す。
func New(ctx context.Context) (FileRepo, func(), error) {
	switch os.Getenv("STATE_BACKEND") {
	case "postgres":
		db, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
		if err != nil {
			return nil, nil, err
		}
		return NewFileRepository(db), db.Close, err
	default:
		client, err := firestore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
		return NewFirestoreFileRepository(client), func() { client.Close() }, err
	}
}
