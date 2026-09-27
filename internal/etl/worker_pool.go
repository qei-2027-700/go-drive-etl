package etl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/parser"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
	"google.golang.org/api/googleapi"
)

var workspaceExportMimeTypes = map[string]string{
	"application/vnd.google-apps.document":     "text/plain",
	"application/vnd.google-apps.spreadsheet":  "text/csv",
	"application/vnd.google-apps.presentation": "application/pdf",
}

const (
	bigQueryLoadMaxAttempts  = 3
	bigQueryLoadInitialDelay = time.Second
)

// waitForBigQueryRetry is a variable so retry behavior can be tested without
// waiting for the production backoff interval.
var waitForBigQueryRetry = waitForContext

type WorkerPool interface {
	Run(
		ctx context.Context,
		repo repository.FileRepo,
		driveClient drive.DriveClient,
		bqClient bq.BQClient,
	) error
}

func Run(
	ctx context.Context,
	repo repository.FileRepo,
	driveClient drive.DriveClient,
	bqClient bq.BQClient,
) error {
	markdownParser := parser.MarkdownParser{}

	files, err := repo.ListPending(ctx)
	if err != nil {
		return err
	}

	jobs := make(chan *domain.File, 100)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case file, ok := <-jobs:
					if !ok {
						return
					}

					var content []byte
					var err error
					if exportMimeType, ok := workspaceExportMimeTypes[file.MimeType]; ok {
						// Google Docs / Sheets / Slides
						content, err = driveClient.DownloadGoogleWorkspaceFile(
							ctx,
							file.DriveFileID,
							exportMimeType,
						)
					} else {
						// PDF / CSV / JSONなど
						content, err = driveClient.DownloadFile(ctx, file.DriveFileID)
					}
					if err != nil {
						if ctx.Err() != nil {
							return
						}

						log.Printf("DownloadFile failed: fileID=%s err=%v", file.DriveFileID, err)

						if statusErr := repo.UpdateStatus(ctx, file.DriveFileID, domain.SyncStatusFailed); statusErr != nil {
							log.Printf("UpdateStatus failed: fileID=%s err=%v", file.DriveFileID, statusErr)
						}
						continue
					}

					if file.MimeType == "text/markdown" {
						chunks, err := markdownParser.Parse(bytes.NewReader(content))
						if err != nil {
							log.Printf("Markdown parse failed: fileID=%s err=%v", file.DriveFileID, err)
							if statusErr := repo.UpdateStatus(ctx, file.DriveFileID, domain.SyncStatusFailed); statusErr != nil {
								log.Printf("UpdateStatus failed: fileID=%s err=%v", file.DriveFileID, statusErr)
							}
							continue
						}

						contentVersion := fmt.Sprintf("%x", sha256.Sum256(content))
						ingestedAt := time.Now().UTC()
						chunkRows := make([]map[string]bigquery.Value, 0, max(1, len(chunks)))
						for index, chunk := range chunks {
							chunkRows = append(chunkRows, map[string]bigquery.Value{
								"file_id":          file.DriveFileID,
								"chunk_index":      index,
								"content":          chunk,
								"embedding_status": "pending",
								"content_version":  contentVersion,
								"ingested_at":      ingestedAt,
								"is_deleted":       false,
							})
						}
						if len(chunkRows) == 0 {
							chunkRows = append(chunkRows, map[string]bigquery.Value{
								"file_id":          file.DriveFileID,
								"chunk_index":      0,
								"content":          "",
								"embedding_status": "pending",
								"content_version":  contentVersion,
								"ingested_at":      ingestedAt,
								"is_deleted":       true,
							})
						}
						if err := insertRowsWithRetry(ctx, bqClient, "chunks", chunkRows); err != nil {
							if ctx.Err() != nil {
								return
							}
							log.Printf("BigQuery chunks InsertRows failed: fileID=%s err=%v", file.DriveFileID, err)
							if statusErr := repo.UpdateStatus(ctx, file.DriveFileID,
								domain.SyncStatusFailed); statusErr != nil {
								log.Printf("UpdateStatus failed: fileID=%s err=%v", file.DriveFileID, statusErr)
							}
							continue
						}
					}

					rows := []map[string]bigquery.Value{
						{
							"drive_file_id": file.DriveFileID,
							"path":          file.Path,
							"checksum":      file.Checksum,
							"mime_type":     file.MimeType,
							"sync_status":   string(domain.SyncStatusDone),
							"updated_at":    time.Now().UTC(),
						},
					}
					if err := insertRowsWithRetry(ctx, bqClient, "drive_files", rows); err != nil {
						if ctx.Err() != nil {
							return
						}

						log.Printf("BigQuery InsertRows failed: fileID=%s err=%v", file.DriveFileID, err)
						if statusErr := repo.UpdateStatus(ctx, file.DriveFileID, domain.SyncStatusFailed); statusErr != nil {
							log.Printf("UpdateStatus failed: fileID=%s err=%v", file.DriveFileID, statusErr)
						}
						continue
					}

					if err := repo.UpdateStatus(ctx, file.DriveFileID, domain.SyncStatusDone); err != nil {
						log.Printf("UpdateStatus failed: fileID=%s err=%v", file.DriveFileID, err)
					}

				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for _, f := range files {
		select {
		case jobs <- f:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		}
	}
	close(jobs)

	wg.Wait()
	return nil
}

// insertRowsWithRetry retries only transient HTTP failures from BigQuery Load
// Jobs. Context cancellation and all non-5xx errors are returned immediately.
func insertRowsWithRetry(
	ctx context.Context,
	client bq.BQClient,
	table string,
	rows []map[string]bigquery.Value,
) error {
	var lastErr error
	for attempt := 1; attempt <= bigQueryLoadMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = client.InsertRows(ctx, table, rows)
		if lastErr == nil {
			return nil
		}
		if !isRetryableBigQueryError(lastErr) {
			log.Printf("BigQuery Load Job failed without retry: table=%s attempts=%d err=%v", table, attempt, lastErr)
			return lastErr
		}
		if attempt == bigQueryLoadMaxAttempts {
			break
		}

		delay := bigQueryLoadInitialDelay * time.Duration(1<<(attempt-1))
		log.Printf(
			"BigQuery Load Job retrying: table=%s retry=%d/%d wait=%s err=%v",
			table,
			attempt,
			bigQueryLoadMaxAttempts-1,
			delay,
			lastErr,
		)
		if err := waitForBigQueryRetry(ctx, delay); err != nil {
			log.Printf("BigQuery Load Job retry interrupted: table=%s retries=%d err=%v", table, attempt, err)
			return err
		}
	}

	log.Printf(
		"BigQuery Load Job failed after retries: table=%s attempts=%d err=%v",
		table,
		bigQueryLoadMaxAttempts,
		lastErr,
	)
	return lastErr
}

func isRetryableBigQueryError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}

	switch apiErr.Code {
	case 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

func waitForContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
