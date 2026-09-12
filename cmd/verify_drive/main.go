package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/joho/godotenv"
	bqclient "github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
)

// log.Fatalf は os.Exit を呼ぶため defer が走らない。
// 後始末を確実に実行するため、処理は run に寄せて main ではエラーを受けるだけにする。
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	_ = godotenv.Load()

	ctx := context.Background()

	// ① Drive: ファイル一覧取得
	driveClient, err := drive.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("Drive クライアントの初期化に失敗: %w", err)
	}

	folderID := os.Getenv("DRIVE_FOLDER_ID")
	files, err := driveClient.ListFiles(ctx, folderID)
	if err != nil {
		return fmt.Errorf("ファイル一覧の取得に失敗: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("ファイルが見つかりませんでした。")
		return nil
	}

	fmt.Printf("Drive 取得ファイル数: %d\n\n", len(files))

	// ② 状態管理DB: Upsert
	repo, closeRepo, err := repository.New(ctx)
	if err != nil {
		return fmt.Errorf("リポジトリ初期化に失敗: %w", err)
	}
	defer closeRepo()

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
		fmt.Printf("  ✓ 状態管理DB Upsert: %s\n", f.Name)
	}

	pending, err := repo.ListPending(ctx)
	if err != nil {
		return fmt.Errorf("ListPending 失敗: %w", err)
	}
	fmt.Printf("  状態管理DB pending 件数: %d\n\n", len(pending))

	// ③ BigQuery: Insert
	bqClient, err := bqclient.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("BigQuery クライアントの初期化に失敗: %w", err)
	}
	defer bqClient.Close()

	var rows []map[string]bigquery.Value
	for _, f := range files {
		rows = append(rows, map[string]bigquery.Value{
			"drive_file_id": f.Id,
			"path":          f.Name,
			"checksum":      f.Md5Checksum,
			"mime_type":     f.MimeType,
			"sync_status":   string(domain.SyncStatusPending),
			"updated_at":    time.Now().UTC(),
		})
	}

	if err := bqClient.InsertRows(ctx, "drive_files", rows); err != nil {
		return fmt.Errorf("BigQuery Insert 失敗: %w", err)
	}

	fmt.Printf("  ✓ BigQuery Insert: %d 件\n", len(rows))
	fmt.Println("\n--- Drive → 状態管理DB → BigQuery 疎通完了 ---")

	return nil
}
