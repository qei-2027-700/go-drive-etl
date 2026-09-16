// Command worker synchronizes files from Google Drive into the ETL pipeline.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	bqclient "github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/etl"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	driveClient, err := drive.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("Drive クライアントの初期化に失敗: %w", err)
	}

	repo, closeRepo, err := repository.New(ctx)
	if err != nil {
		return fmt.Errorf("リポジトリの初期化に失敗: %w", err)
	}
	defer closeRepo()

	bq, err := bqclient.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("BigQuery クライアントの初期化に失敗: %w", err)
	}
	defer bq.Close()

	files, err := driveClient.ListFiles(ctx, os.Getenv("DRIVE_FOLDER_ID"))
	if err != nil {
		return fmt.Errorf("Drive ファイル一覧の取得に失敗: %w", err)
	}

	for _, file := range files {
		record := &domain.File{
			DriveFileID: file.Id,
			Path:        file.Name,
			Checksum:    file.Md5Checksum,
			MimeType:    file.MimeType,
			SyncStatus:  domain.SyncStatusPending,
		}
		if err := repo.Upsert(ctx, record); err != nil {
			log.Printf("状態管理 DB への Upsert に失敗: file=%q id=%s err=%v", file.Name, file.Id, err)
		}
	}

	log.Printf("Drive ファイルを %d 件検出しました。Worker Pool を開始します。", len(files))
	if err := etl.Run(ctx, repo, driveClient, bq); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println("停止シグナルを受信しました。安全にシャットダウンします。")
			return nil
		}
		return fmt.Errorf("Worker Pool の実行に失敗: %w", err)
	}

	log.Println("ETL パイプラインが完了しました。")
	return nil
}
