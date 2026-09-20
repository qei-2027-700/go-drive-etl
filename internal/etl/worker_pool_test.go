package etl

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/bigquery"
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

// ListPendingでファイルが1件帰ってきたとき、ワーカーがダウンロードして BigQuery へロードすること
func TestRun_DownloadAndLoadFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	ctx := context.Background()

	file := &domain.File{
		DriveFileID: "test-drive-id",
		Path:        "report.csv",
		Checksum:    "checksum",
		MimeType:    "text/csv",
	}

	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{file}, nil)

	driveClient.EXPECT().
		DownloadFile(gomock.Any(), "test-drive-id").
		Return([]byte("data"), nil)
	bqClient.EXPECT().
		InsertRows(gomock.Any(), "drive_files", gomock.Any()).
		DoAndReturn(func(_ context.Context, table string, rows []map[string]bigquery.Value) error {
			if len(rows) != 1 {
				t.Fatalf("expected one row, got %d", len(rows))
			}
			if rows[0]["drive_file_id"] != file.DriveFileID || rows[0]["path"] != file.Path {
				t.Fatalf("unexpected row: %#v", rows[0])
			}
			if rows[0]["sync_status"] != string(domain.SyncStatusDone) {
				t.Fatalf("expected done status, got %#v", rows[0]["sync_status"])
			}
			return nil
		})
	repo.EXPECT().
		UpdateStatus(gomock.Any(), "test-drive-id", domain.SyncStatusDone).
		Return(nil)

	err := Run(ctx, repo, driveClient, bqClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Google Docsの場合は、DownloadFileではなく、text/plain で
// DownloadGoogleWorkspaceFile を呼ぶこと
func TestRun_ExportGoogleDocument(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	file := &domain.File{
		DriveFileID: "google-doc-id",
		Path:        "meeting-notes",
		Checksum:    "",
		MimeType:    "application/vnd.google-apps.document",
	}

	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{file}, nil)

	// 今回確認したい箇所
	driveClient.EXPECT().
		DownloadGoogleWorkspaceFile(
			gomock.Any(),
			"google-doc-id",
			"text/plain",
		).
		Return([]byte("document contents"), nil)

	bqClient.EXPECT().
		InsertRows(gomock.Any(), "drive_files", gomock.Any()).
		Return(nil)

	repo.EXPECT().
		UpdateStatus(gomock.Any(), "google-doc-id", domain.SyncStatusDone).
		Return(nil)

	err := Run(context.Background(), repo, driveClient, bqClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// BigQuery へのロードが失敗したとき、ステータスが SyncStatusFailed に更新されること
func TestRun_LoadFile_Failed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	file := &domain.File{DriveFileID: "test-drive-id"}
	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte("data"), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).Return(errors.New("BigQuery error"))
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusFailed).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
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
		DriveFileID: "test-drive-id",
	}

	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{file}, nil)

	driveClient.EXPECT().
		DownloadFile(gomock.Any(), "test-drive-id").
		Return(nil, errors.New("network error"))

	repo.EXPECT().
		UpdateStatus(gomock.Any(), "test-drive-id", domain.SyncStatusFailed).
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
		files[i] = &domain.File{DriveFileID: fmt.Sprintf("drive-id-%d", i)}
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
