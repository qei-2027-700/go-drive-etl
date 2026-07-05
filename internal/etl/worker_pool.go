package etl

import (
	"context"
	"log"
	"sync"

	"github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
)

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
					_, err := driveClient.DownloadFile(ctx, file.DriveFileID)
					if err != nil {
						if ctx.Err() != nil {
							return
						}
						log.Printf("DownloadFile failed: fileID=%s err=%v", file.DriveFileID, err)
						if statusErr := repo.UpdateStatus(ctx, file.ID, domain.SyncStatusFailed); statusErr != nil {
							log.Printf("UpdateStatus failed: fileID=%s err=%v", file.DriveFileID, statusErr)
						}
						continue
					}
					// TODO: BQ 保存 + UpdateStatus(Done) は Phase 2 で実装する

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
