package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"go.uber.org/mock/gomock"
	driveapi "google.golang.org/api/drive/v3"

	bqMock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	driveMock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
	repoMock "github.com/qei-2027-700/go-drive-etl/internal/repository/mock"
)

func TestRunPipeline_WithoutReprocessDoesNotRequeue(t *testing.T) {
	t.Setenv("REPROCESS_ALL", "")

	ctrl := gomock.NewController(t)
	repo := repoMock.NewMockFileRepo(ctrl)
	drive := driveMock.NewMockDriveClient(ctrl)
	bq := bqMock.NewMockBQClient(ctrl)

	drive.EXPECT().ListFiles(gomock.Any(), "folder-id").
		Return([]*driveapi.File{{Id: "file-id", Name: "memo.md"}}, nil)
	repo.EXPECT().Upsert(gomock.Any(), &domain.File{
		DriveFileID: "file-id",
		Path:        "memo.md",
		SyncStatus:  domain.SyncStatusPending,
	}).Return(nil)

	// Normal discovery must not force completed files back to pending.
	repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	// Isolate requeue behavior: there is no pending work to download or load.
	repo.EXPECT().ListPending(gomock.Any()).Return(nil, nil)

	if err := runPipeline(context.Background(), repo, drive, bq, "folder-id", ""); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
}

func TestRunPipeline_ReprocessFalseDoesNotRequeue(t *testing.T) {
	t.Setenv("REPROCESS_ALL", "false")
	ctrl := gomock.NewController(t)
	repo := repoMock.NewMockFileRepo(ctrl)
	drive := driveMock.NewMockDriveClient(ctrl)
	bq := bqMock.NewMockBQClient(ctrl)

	drive.EXPECT().ListFiles(gomock.Any(), "folder-id").
		Return([]*driveapi.File{{Id: "file-id", Name: "memo.md"}}, nil)
	repo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	repo.EXPECT().ListPending(gomock.Any()).Return(nil, nil)

	if err := runPipeline(context.Background(), repo, drive, bq, "folder-id", ""); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
}

func TestRunPipeline_ReprocessAllRequeuesEveryDiscoveredFile(t *testing.T) {
	t.Setenv("REPROCESS_ALL", "true")
	ctrl := gomock.NewController(t)
	repo := repoMock.NewMockFileRepo(ctrl)
	drive := driveMock.NewMockDriveClient(ctrl)
	bq := bqMock.NewMockBQClient(ctrl)
	files := []*driveapi.File{
		{Id: "file-a", Name: "first.md", Md5Checksum: "checksum-a", MimeType: "text/markdown"},
		{Id: "file-b", Name: "second.md", Md5Checksum: "checksum-b", MimeType: "text/markdown"},
	}
	drive.EXPECT().ListFiles(gomock.Any(), "folder-id").Return(files, nil)
	for _, file := range files {
		// Each file must be registered before its own status is reset.
		gomock.InOrder(
			repo.EXPECT().Upsert(gomock.Any(), &domain.File{
				DriveFileID: file.Id,
				Path:        file.Name,
				Checksum:    file.Md5Checksum,
				MimeType:    file.MimeType,
				SyncStatus:  domain.SyncStatusPending,
			}).Return(nil),
			repo.EXPECT().UpdateStatus(gomock.Any(), file.Id, domain.SyncStatusPending).Return(nil),
		)
	}
	repo.EXPECT().ListPending(gomock.Any()).Return(nil, nil)

	if err := runPipeline(context.Background(), repo, drive, bq, "folder-id", ""); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
}

func TestRunPipeline_ReprocessStatusFailureReportsFailure(t *testing.T) {
	t.Setenv("REPROCESS_ALL", "true")
	ctrl := gomock.NewController(t)
	repo := repoMock.NewMockFileRepo(ctrl)
	drive := driveMock.NewMockDriveClient(ctrl)
	bq := bqMock.NewMockBQClient(ctrl)
	wantErr := errors.New("state update failed")

	drive.EXPECT().ListFiles(gomock.Any(), "folder-id").
		Return([]*driveapi.File{{Id: "file-id", Name: "memo.md"}}, nil)
	gomock.InOrder(
		repo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil),
		repo.EXPECT().UpdateStatus(gomock.Any(), "file-id", domain.SyncStatusPending).Return(wantErr),
	)
	repo.EXPECT().ListPending(gomock.Any()).Return(nil, nil)

	err := runPipeline(context.Background(), repo, drive, bq, "folder-id", "")
	if err == nil {
		t.Fatal("runPipeline succeeded after a state update failure")
	}
	if strings.Contains(err.Error(), "file-id") || strings.Contains(err.Error(), wantErr.Error()) {
		t.Errorf("error exposes file or backend details: %v", err)
	}
}

func TestRunPipeline_ReprocessReportsFailedUpsertAfterOtherFiles(t *testing.T) {
	t.Setenv("REPROCESS_ALL", "true")
	ctrl := gomock.NewController(t)
	repo := repoMock.NewMockFileRepo(ctrl)
	drive := driveMock.NewMockDriveClient(ctrl)
	bq := bqMock.NewMockBQClient(ctrl)

	drive.EXPECT().ListFiles(gomock.Any(), "folder-id").Return([]*driveapi.File{
		{Id: "failed-file", Name: "failed.md"},
		{Id: "next-file", Name: "next.md"},
	}, nil)
	gomock.InOrder(
		repo.EXPECT().Upsert(gomock.Any(), &domain.File{
			DriveFileID: "failed-file", Path: "failed.md", SyncStatus: domain.SyncStatusPending,
		}).Return(errors.New("registration failed")),
		repo.EXPECT().Upsert(gomock.Any(), &domain.File{
			DriveFileID: "next-file", Path: "next.md", SyncStatus: domain.SyncStatusPending,
		}).Return(nil),
		repo.EXPECT().UpdateStatus(gomock.Any(), "next-file", domain.SyncStatusPending).Return(nil),
		repo.EXPECT().ListPending(gomock.Any()).Return(nil, nil),
	)
	repo.EXPECT().UpdateStatus(gomock.Any(), "failed-file", gomock.Any()).Times(0)

	if err := runPipeline(context.Background(), repo, drive, bq, "folder-id", "exports"); err == nil {
		t.Fatal("runPipeline succeeded after a registration failure")
	}
}

func TestRunPipeline_DoesNotExportAfterWorkerFailure(t *testing.T) {
	t.Setenv("REPROCESS_ALL", "")
	ctrl := gomock.NewController(t)
	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &driveapi.File{Id: "file-id", Name: "memo.md"}

	driveClient.EXPECT().ListFiles(gomock.Any(), "folder-id").Return([]*driveapi.File{file}, nil)
	repo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{{DriveFileID: file.Id}}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.Id).Return(nil, errors.New("private API detail"))
	repo.EXPECT().UpdateStatus(gomock.Any(), file.Id, domain.SyncStatusFailed).Return(nil)
	// No CSV export expectations: an attempted export fails the test.
	if err := runPipeline(context.Background(), repo, driveClient, bqClient, "folder-id", "exports"); err == nil {
		t.Fatal("runPipeline succeeded after a worker failure")
	}
}
