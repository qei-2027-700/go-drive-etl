package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	bqclient "github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()

	// ① Drive: ファイル一覧取得
	driveClient, err := drive.NewClient(ctx)
	if err != nil {
		log.Fatalf("Drive クライアントの初期化に失敗: %v", err)
	}

	folderID := os.Getenv("DRIVE_FOLDER_ID")
	files, err := driveClient.ListFiles(ctx, folderID)
	if err != nil {
		log.Fatalf("ファイル一覧の取得に失敗: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("ファイルが見つかりませんでした。")
		return
	}

	fmt.Printf("Drive 取得ファイル数: %d\n\n", len(files))

	// ② PostgreSQL: Upsert
	db, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
	if err != nil {
		log.Fatalf("DB 接続に失敗: %v", err)
	}
	defer db.Close()

	repo := repository.NewFileRepository(db)

	for _, f := range files {
		record := &domain.File{
			DriveFileID: f.Id,
			Path:        f.Name,
			Checksum:    f.Md5Checksum,
			MimeType:    f.MimeType,
			SyncStatus:  domain.SyncStatusPending,
		}
		if err := repo.Upsert(ctx, record); err != nil {
			log.Printf("Upsert 失敗 [%s]: %v", f.Name, err)
			continue
		}
		fmt.Printf("  ✓ PostgreSQL Upsert: %s\n", f.Name)
	}

	pending, err := repo.ListPending(ctx)
	if err != nil {
		log.Fatalf("ListPending 失敗: %v", err)
	}
	fmt.Printf("  PostgreSQL pending 件数: %d\n\n", len(pending))

	// ③ BigQuery: Insert
	bqClient, err := bqclient.NewClient(ctx)
	if err != nil {
		log.Fatalf("BigQuery クライアントの初期化に失敗: %v", err)
	}
	defer bqClient.Close()

	var rows []map[string]bigquery.Value
	for _, f := range files {
		rows = append(rows, map[string]bigquery.Value{
			"drive_file_id": f.Id,
			"name":          f.Name,
			"mime_type":     f.MimeType,
			"checksum":      f.Md5Checksum,
			"ingested_at":   time.Now().UTC(),
		})
	}

	if err := bqClient.InsertRows(ctx, "files", rows); err != nil {
		log.Fatalf("BigQuery Insert 失敗: %v", err)
	}

	fmt.Printf("  ✓ BigQuery Insert: %d 件\n", len(rows))
	fmt.Println("\n--- Drive → PostgreSQL → BigQuery 疎通完了 ---")
}
