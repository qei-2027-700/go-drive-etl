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
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	bqclient "github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/etl"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
)

func main() {
	_ = godotenv.Load()
	metadata := newRunMetadata()
	if err := sentry.Init(sentryOptions(metadata)); err != nil {
		log.Fatalf("Sentry の初期化に失敗: %v", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			captureFailure(recoveredPanicError(recovered), metadata)
			sentry.Flush(2 * time.Second)
			panic(recovered)
		}
	}()

	if err := run(); err != nil {
		captureFailure(err, metadata)
		sentry.Flush(2 * time.Second)
		log.Print(err)
		os.Exit(1)
	}
	sentry.Flush(2 * time.Second)
}

func run() error {
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	driveClient, err := drive.NewClient(ctx)
	if err != nil {
		return withStage("startup", fmt.Errorf("Drive クライアントの初期化に失敗: %w", err))
	}

	repo, closeRepo, err := repository.New(ctx)
	if err != nil {
		return withStage("startup", fmt.Errorf("リポジトリの初期化に失敗: %w", err))
	}
	defer closeRepo()

	bq, err := bqclient.NewClient(ctx)
	if err != nil {
		return withStage("startup", fmt.Errorf("BigQuery クライアントの初期化に失敗: %w", err))
	}
	defer bq.Close()

	if err := runPipeline(ctx, repo, driveClient, bq, os.Getenv("DRIVE_FOLDER_ID"), os.Getenv("DRIVE_EXPORT_FOLDER_ID")); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println("停止シグナルを受信しました。安全にシャットダウンします。")
			return nil
		}
		return fmt.Errorf("ETL パイプラインの実行に失敗: %w", err)
	}

	log.Println("ETL パイプラインが完了しました。")
	return nil
}

// runPipeline discovers files, registers them in the state store, then runs
// the worker pool. Keeping the orchestration separate from client construction
// makes the production entry point integration-testable with real state storage
// and mocked external APIs.
func runPipeline(
	ctx context.Context,
	repo repository.FileRepo,
	driveClient drive.DriveClient,
	bq bqclient.BQClient,
	folderID string,
	exportFolderID string,
) error {
	files, err := driveClient.ListFiles(ctx, folderID)
	if err != nil {
		return withStage("download", fmt.Errorf("Drive ファイル一覧の取得に失敗: %w", err))
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
		return withStage("load", fmt.Errorf("Worker Pool の実行に失敗: %w", err))
	}
	return exportGoldReports(ctx, driveClient, bq, exportFolderID)
}

func exportGoldReports(ctx context.Context, driveClient drive.DriveClient, bq bqclient.BQClient, exportFolderID string) error {
	if exportFolderID == "" {
		log.Println("DRIVE_EXPORT_FOLDER_ID が未設定のため Gold CSV の出力をスキップします。")
		return nil
	}
	for _, table := range []string{"mart_ingestion_daily", "mart_file_latest"} {
		contents, err := bq.ExportTableCSV(ctx, table)
		if err != nil {
			return withStage("load", fmt.Errorf("Gold Mart の CSV 抽出に失敗: table=%s: %w", table, err))
		}
		name := table + ".csv"
		if err := driveClient.UpsertFile(ctx, exportFolderID, name, "text/csv", contents); err != nil {
			return withStage("load", fmt.Errorf("Gold Mart CSV の Drive 出力に失敗: file=%s: %w", name, err))
		}
	}
	return nil
}
