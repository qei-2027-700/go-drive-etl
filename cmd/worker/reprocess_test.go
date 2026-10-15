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

func TestRunPipeline_ReprocessStatusFailureStopsPipeline(t *testing.T) {
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
	repo.EXPECT().ListPending(gomock.Any()).Times(0)

	err := runPipeline(context.Background(), repo, drive, bq, "folder-id", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "file-id") {
		t.Errorf("error does not identify the failed file: %v", err)
	}
}

func TestRunPipeline_ReprocessSkipsFailedUpsertAndContinues(t *testing.T) {
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

	if err := runPipeline(context.Background(), repo, drive, bq, "folder-id", ""); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
}
