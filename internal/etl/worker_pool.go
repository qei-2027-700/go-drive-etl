package etl

import (
	"bytes"
	"context"
	"crypto/sha256"
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
)

var workspaceExportMimeTypes = map[string]string{
	"application/vnd.google-apps.document":     "text/plain",
	"application/vnd.google-apps.spreadsheet":  "text/csv",
	"application/vnd.google-apps.presentation": "application/pdf",
}

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

						chunkRows := make([]map[string]bigquery.Value, 0, len(chunks))
						for index, chunk := range chunks {
							chunkRows = append(chunkRows, map[string]bigquery.Value{
								"file_id":          file.DriveFileID,
								"chunk_index":      index,
								"content":          chunk,
								"embedding_status": "pending",
							})
						}

						contentVersion := fmt.Sprintf("%x", sha256.Sum256(content))
						if err := bqClient.ReplaceChunkRows(ctx, file.DriveFileID, contentVersion, chunkRows); err != nil {
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
					if err := bqClient.InsertRows(ctx, "drive_files", rows); err != nil {
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
