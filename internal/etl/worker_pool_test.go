package etl

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/googleapi"

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

func TestRun_MarkdownFileStoresChunks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{
		DriveFileID: "markdown-file-id",
		Path:        "guide.md",
		MimeType:    "text/markdown",
	}

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).
		Return([]byte("# Overview\nSummary.\n\n## Steps\nDo this."), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, rows []map[string]bigquery.Value) error {
			wantVersion := fmt.Sprintf("%x", sha256.Sum256([]byte("# Overview\nSummary.\n\n## Steps\nDo this.")))
			if len(rows) != 2 {
				t.Fatalf("chunk row count = %d, want 2", len(rows))
			}
			if rows[0]["content_version"] != wantVersion || rows[0]["is_deleted"] != false {
				t.Errorf("unexpected chunk metadata: %#v", rows[0])
			}
			if rows[0]["file_id"] != file.DriveFileID || rows[0]["chunk_index"] != 0 || rows[0]["content"] != "# Overview\nSummary." {
				t.Errorf("unexpected first chunk row: %#v", rows[0])
			}
			if rows[1]["chunk_index"] != 1 || rows[1]["content"] != "## Steps\nDo this." {
				t.Errorf("unexpected second chunk row: %#v", rows[1])
			}
			return nil
		})
	bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).Return(nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusDone).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRun_MarkdownChunkInsertFailureMarksFileFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{DriveFileID: "markdown-file-id", MimeType: "text/markdown"}

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte("# Overview\nSummary."), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).Return(errors.New("BigQuery error"))
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusFailed).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRun_MarkdownChunkLoadRetriesTransientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{DriveFileID: "markdown-file-id", MimeType: "text/markdown"}

	originalWait := waitForBigQueryRetry
	waitForBigQueryRetry = func(_ context.Context, delay time.Duration) error {
		if delay != bigQueryLoadInitialDelay {
			t.Errorf("first retry delay = %s, want %s", delay, bigQueryLoadInitialDelay)
		}
		return nil
	}
	t.Cleanup(func() { waitForBigQueryRetry = originalWait })

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte("# Overview\nSummary."), nil)

	chunkAttempts := 0
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).Times(2).
		DoAndReturn(func(_ context.Context, _ string, _ []map[string]bigquery.Value) error {
			chunkAttempts++
			if chunkAttempts == 1 {
				return &googleapi.Error{Code: 503, Message: "Service Unavailable"}
			}
			return nil
		})
	bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).Return(nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusDone).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRun_DoesNotRetryNonRetryableBigQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{DriveFileID: "markdown-file-id", MimeType: "text/markdown"}

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte("# Overview\nSummary."), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).Times(1).
		Return(&googleapi.Error{Code: 400, Message: "Bad Request"})
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusFailed).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRun_MarksFailedAfterTransientRetriesAreExhausted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{DriveFileID: "markdown-file-id", MimeType: "text/markdown"}

	originalWait := waitForBigQueryRetry
	waitForBigQueryRetry = func(_ context.Context, _ time.Duration) error { return nil }
	t.Cleanup(func() { waitForBigQueryRetry = originalWait })

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte("# Overview\nSummary."), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).Times(bigQueryLoadMaxAttempts).
		Return(&googleapi.Error{Code: 503, Message: "Service Unavailable"})
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusFailed).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRun_DoesNotRetryDriveFilesLoad(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{DriveFileID: "test-drive-id"}

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte("data"), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).Times(1).
		Return(&googleapi.Error{Code: 503, Message: "Service Unavailable"})
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusFailed).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestInsertRowsWithRetry_DoesNotRetryContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bqClient := bqMock.NewMockBQClient(ctrl)
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).Times(1).Return(context.Canceled)

	err := insertRowsWithRetry(context.Background(), bqClient, "chunks", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("insertRowsWithRetry() error = %v, want context.Canceled", err)
	}
}

func TestIsRetryableBigQueryError(t *testing.T) {
	for _, code := range []int{500, 502, 503, 504} {
		if !isRetryableBigQueryError(&googleapi.Error{Code: code}) {
			t.Errorf("HTTP %d should be retryable", code)
		}
	}
	for _, err := range []error{
		&googleapi.Error{Code: 400},
		errors.New("network error"),
		context.Canceled,
		context.DeadlineExceeded,
	} {
		if isRetryableBigQueryError(err) {
			t.Errorf("%v should not be retryable", err)
		}
	}
}

func TestRun_EmptyMarkdownStoresTombstone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	file := &domain.File{DriveFileID: "markdown-file-id", MimeType: "text/markdown"}

	repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return([]byte(" \n\n"), nil)
	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, rows []map[string]bigquery.Value) error {
			if len(rows) != 1 || rows[0]["is_deleted"] != true || rows[0]["content"] != "" {
				t.Errorf("expected one tombstone row, got %#v", rows)
			}
			return nil
		})
	bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).Return(nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusDone).Return(nil)

	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run() error = %v", err)
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

// ここからは、振り返り文書のメタデータを各チャンクへ保存する処理のテスト。
// 最初のテストでは対象期間を持たない文書を使う。
func TestRun_RetroMetadataOnEveryChunk(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantCount int
		deleted   bool
	}{
		{"heading sections", "# Retro\nSummary.\n\n## Actions\nNext step.", 2, false},
		{"long section", "# Retro\n" + strings.Repeat("a", 4100), 2, false},
		{"empty document tombstone", " \n\t", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := &domain.File{
				DriveFileID: "synthetic-retro-id",
				Path:        "retro.md",
				Checksum:    "synthetic-checksum",
				MimeType:    "text/markdown",
			}
			rows := runRetroAndCaptureRows(t, file, []byte(tc.content))
			if len(rows) != tc.wantCount {
				t.Fatalf("chunk count = %d, want %d", len(rows), tc.wantCount)
			}
			wantVersion := fmt.Sprintf("%x", sha256.Sum256([]byte(tc.content)))
			for i, row := range rows {
				if row["file_id"] != file.DriveFileID || row["chunk_index"] != i ||
					row["content_version"] != wantVersion || row["is_deleted"] != tc.deleted {
					t.Errorf("chunk %d: unexpected identity/version: %#v", i, row)
				}
				if row["document_name"] != file.Path {
					t.Errorf("chunk %d: document_name = %#v, want %q", i, row["document_name"], file.Path)
				}
				wantURL := "https://drive.google.com/file/d/" + file.DriveFileID + "/view"
				if row["drive_url"] != wantURL {
					t.Errorf("chunk %d: drive_url = %#v, want %q", i, row["drive_url"], wantURL)
				}
				if row["ingested_at"] == nil {
					t.Errorf("chunk %d: ingestion timestamp missing", i)
				}
			}
		})
	}
}

func TestRun_RetroWithoutDateKeepsPeriodNull(t *testing.T) {
	file := &domain.File{
		DriveFileID: "synthetic-undated-id",
		Path:        "undated-retro.md",
		MimeType:    "text/markdown",
	}
	rows := runRetroAndCaptureRows(t, file, []byte("# Retro\nSummary.\n\n## Actions\nNext step."))
	if len(rows) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(rows))
	}
	for i, row := range rows {
		// Omitted NDJSON fields and explicit null both represent a missing
		// nullable DATE. Neither today's date nor ingested_at is a target date.
		for _, field := range []string{"retro_start_date", "retro_end_date"} {
			if row[field] != nil {
				t.Errorf("chunk %d: missing %s replaced with %#v", i, field, row[field])
			}
		}
		if row["ingested_at"] == nil {
			t.Errorf("chunk %d: ingestion timestamp missing", i)
		}
	}
}

func TestRun_RetroTitleLabelOnEveryChunk(t *testing.T) {
	t.Setenv("RETRO_SPRINT_FROM_MONTH", "")
	file := &domain.File{
		DriveFileID: "synthetic-monthly-id",
		Path:        "202609-3 synthetic-export-id.md",
		MimeType:    "text/markdown",
	}
	// The first heading, rather than the exported filename, supplies the label.
	rows := runRetroAndCaptureRows(t, file, []byte("# 202610-1\nSummary.\n\n## Actions\nNext step."))
	if len(rows) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(rows))
	}
	for i, row := range rows {
		if row["retro_month"] != "2026-10" || row["retro_sequence"] != int32(1) {
			t.Errorf("chunk %d: label = %v/%v, want 2026-10/1", i, row["retro_month"], row["retro_sequence"])
		}
		for _, field := range []string{"retro_start_date", "retro_end_date"} {
			if row[field] != nil {
				t.Errorf("chunk %d: inferred exact period %s = %v", i, field, row[field])
			}
		}
	}
}

// TestRun_RetroSprintOnEveryChunk は、分割後の全チャンクに同じ対象期間が付くことと、
// 適用開始月を変えて同じ本文を再処理したときに、対象外となった期間が未設定になることを確認する。
func TestRun_RetroSprintOnEveryChunk(t *testing.T) {
	// 月またぎを確認するため、このテスト内だけ適用開始月を2026年9月にする。
	// 本番用の設定例（2026年11月から）を変更するものではない。
	t.Setenv("RETRO_SPRINT_FROM_MONTH", "2026-09")
	file := &domain.File{DriveFileID: "synthetic-sprint", Path: "ignored.md", MimeType: "text/markdown"}
	content := []byte("# 202609-4\nSummary.\n\n## Actions\nNext step.")
	rows := runRetroAndCaptureRows(t, file, content)
	if len(rows) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(rows))
	}
	// 202609-4 は9月の第4月曜日（9/28）から10/4まで。
	// 2つのチャンクが同じ期間を持ち、タイトルの年月は9月のままであることを確認する。
	for i, row := range rows {
		if row["retro_start_date"] != "2026-09-28" || row["retro_end_date"] != "2026-10-04" {
			t.Errorf("chunk %d: unexpected period: %v/%v", i, row["retro_start_date"], row["retro_end_date"])
		}
		if row["retro_month"] != "2026-09" {
			t.Errorf("chunk %d: title month changed", i)
		}
	}
	// 適用開始月を10月へ変更すると、この9月の文書は期間計算の対象外になる。
	// 同じ本文なのでcontent_versionは変わらず、再出力する行の開始日・終了日は未設定になる。
	// 保存済みの旧行が検索から除外されることは、別途BigQueryの合成データで検証する。
	t.Setenv("RETRO_SPRINT_FROM_MONTH", "2026-10")
	after := runRetroAndCaptureRows(t, file, content)
	for i, row := range after {
		if row["content_version"] != rows[i]["content_version"] || row["retro_start_date"] != nil || row["retro_end_date"] != nil {
			t.Errorf("chunk %d: metadata refresh failed", i)
		}
	}
}

func TestRun_InvalidSprintConfigurationStopsBeforeReadingFiles(t *testing.T) {
	t.Setenv("RETRO_SPRINT_FROM_MONTH", "2026-13")
	// No external dependency may be used when configuration is invalid.
	if err := Run(context.Background(), nil, nil, nil); err == nil {
		t.Fatal("invalid configuration should fail")
	}
}

func TestRun_RetroRenamePreservesContentVersionAndUpdatesSource(t *testing.T) {
	content := []byte("# Retro\nSummary.\n\n## Actions\nNext step.")
	file := &domain.File{
		DriveFileID: "synthetic-retro-id",
		Path:        "retro.md",
		Checksum:    "unchanged-checksum",
		MimeType:    "text/markdown",
	}
	before := runRetroAndCaptureRows(t, file, content)
	renamed := *file
	renamed.Path = "corrected-retro.md"
	after := runRetroAndCaptureRows(t, &renamed, content)
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("chunk counts = %d/%d, want 2/2", len(before), len(after))
	}
	for i := range before {
		if before[i]["content_version"] != after[i]["content_version"] || before[i]["content"] != after[i]["content"] {
			t.Errorf("chunk %d: rename changed content identity", i)
		}
		if before[i]["document_name"] != file.Path || after[i]["document_name"] != renamed.Path {
			t.Errorf("chunk %d: names = %#v/%#v, want %q/%q", i,
				before[i]["document_name"], after[i]["document_name"], file.Path, renamed.Path)
		}
		wantURL := "https://drive.google.com/file/d/" + file.DriveFileID + "/view"
		if before[i]["drive_url"] != wantURL || after[i]["drive_url"] != wantURL {
			t.Errorf("chunk %d: source identity lost across rename", i)
		}
	}
}

func runRetroAndCaptureRows(t *testing.T, file *domain.File, content []byte) []map[string]bigquery.Value {
	t.Helper()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)
	var captured []map[string]bigquery.Value
	gomock.InOrder(
		repo.EXPECT().ListPending(gomock.Any()).Return([]*domain.File{file}, nil),
		driveClient.EXPECT().DownloadFile(gomock.Any(), file.DriveFileID).Return(content, nil),
		bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).DoAndReturn(
			func(_ context.Context, _ string, rows []map[string]bigquery.Value) error {
				captured = rows
				return nil
			}),
		bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).Return(nil),
		repo.EXPECT().UpdateStatus(gomock.Any(), file.DriveFileID, domain.SyncStatusDone).Return(nil),
	)
	if err := Run(context.Background(), repo, driveClient, bqClient); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i, row := range captured {
		for _, suffix := range []string{"month", "sequence", "start_date", "end_date"} {
			if _, exists := row["reflection_"+suffix]; exists {
				t.Errorf("chunk %d: obsolete Raw column reflection_%s", i, suffix)
			}
		}
	}
	return captured
}
