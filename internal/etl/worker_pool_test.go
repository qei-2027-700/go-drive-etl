package etl

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"go.uber.org/mock/gomock"

	bqMock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	driveMock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
	repoMock "github.com/qei-2027-700/go-drive-etl/internal/repository/mock"
)

func TestRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	ctx := context.Background()

	// 期待される呼び出しを設定
	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{}, nil)

	// Runを実行
	err := Run(ctx, repo, driveClient, bqClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ListPendingでファイルが1件帰ってきたとき、ワーカーがDownloadFileを呼び出すこと
func TestRun_DownloadFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	ctx := context.Background()

	file := &domain.File{
		ID:          1,
		DriveFileID: "test-drive-id",
	}

	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{file}, nil)

	driveClient.EXPECT().
		DownloadFile(gomock.Any(), "test-drive-id").
		Return([]byte("data"), nil)

	err := Run(ctx, repo, driveClient, bqClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// DownloadFile が失敗したとき、ステータスが SyncStatusFailed に更新されること
func TestRun_DownloadFile_Failed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	ctx := context.Background()

	file := &domain.File{
		ID:          1,
		DriveFileID: "test-drive-id",
	}

	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{file}, nil)

	driveClient.EXPECT().
		DownloadFile(gomock.Any(), "test-drive-id").
		Return(nil, errors.New("network error"))

	repo.EXPECT().
		UpdateStatus(gomock.Any(), int64(1), domain.SyncStatusFailed).
		Return(nil)

	err := Run(ctx, repo, driveClient, bqClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// コンテキストがキャンセルされたとき、Run が context.Canceled を返すこと
func TestRun_CtxCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// バッファ(100)を超える件数でキャンセルパスを確実に通す
	files := make([]*domain.File, 101)
	for i := range files {
		files[i] = &domain.File{ID: int64(i + 1), DriveFileID: fmt.Sprintf("drive-id-%d", i)}
	}

	repo.EXPECT().ListPending(gomock.Any()).Return(files, nil)
	// ワーカーがキャンセル前にいくつか処理する可能性があるため AnyTimes で許容
	driveClient.EXPECT().DownloadFile(gomock.Any(), gomock.Any()).
		Return(nil, context.Canceled).AnyTimes()
	repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	err := Run(ctx, repo, driveClient, bqClient)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
