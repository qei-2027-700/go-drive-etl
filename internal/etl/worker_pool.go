package etl

import (
	"context"
	"sync"

	"github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
)

func Run(
	ctx context.Context,
	repo *repository.FileRepository,
	driveClient *drive.Client,
	bqClient *bq.Client,
) error {
	//
	files, err := repo.ListPending(ctx)
	if err != nil {
		return err
	}

	// job チャネルを作成
	jobs := make(chan *domain.File, 100)

	// 3. ５つのworker を起動
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
					// Drive からDL -> BQに保存
					repo.UpdateStatus(ctx, file.ID, domain.SyncStatusDone)

				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// 4. ファイルをキューに入れる
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	// 5. 全 worker の終了を待つ
	wg.Wait()
	return nil
}
