package repository

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New は STATE_BACKEND に応じた FileRepo と、その後始末をする関数を返す。
// 未設定のときは Firestore を使う。未知の値は誤って本番 Firestore へ繋がないようエラーにする。
func New(ctx context.Context) (FileRepo, func(), error) {
	backend := os.Getenv("STATE_BACKEND")

	switch backend {
	case "postgres":
		db, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
		if err != nil {
			return nil, nil, err
		}
		return NewFileRepository(db), db.Close, nil

	case "", "firestore":
		client, err := firestore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
		if err != nil {
			return nil, nil, err
		}
		return NewFirestoreFileRepository(client), func() { _ = client.Close() }, nil

	default:
		return nil, nil, fmt.Errorf("STATE_BACKEND が不正: %q（postgres | firestore | 未設定）", backend)
	}
}
