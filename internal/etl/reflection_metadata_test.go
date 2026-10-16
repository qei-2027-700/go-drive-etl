package etl

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"go.uber.org/mock/gomock"

	bqMock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	driveMock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
	repoMock "github.com/qei-2027-700/go-drive-etl/internal/repository/mock"
)

// These fixtures intentionally have no target period.
func TestRun_ReflectionMetadataOnEveryChunk(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantCount int
		deleted   bool
	}{
		{"heading sections", "# Reflection\nSummary.\n\n## Actions\nNext step.", 2, false},
		{"long section", "# Reflection\n" + strings.Repeat("a", 4100), 2, false},
		{"empty document tombstone", " \n\t", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := &domain.File{
				DriveFileID: "synthetic-reflection-id",
				Path:        "reflection.md",
				Checksum:    "synthetic-checksum",
				MimeType:    "text/markdown",
			}
			rows := runReflectionAndCaptureRows(t, file, []byte(tc.content))
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

func TestRun_ReflectionWithoutDateKeepsPeriodNull(t *testing.T) {
	file := &domain.File{
		DriveFileID: "synthetic-undated-id",
		Path:        "undated-reflection.md",
		MimeType:    "text/markdown",
	}
	rows := runReflectionAndCaptureRows(t, file, []byte("# Reflection\nSummary.\n\n## Actions\nNext step."))
	if len(rows) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(rows))
	}
	for i, row := range rows {
		// Omitted NDJSON fields and explicit null both represent a missing
		// nullable DATE. Neither today's date nor ingested_at is a target date.
		for _, field := range []string{"reflection_start_date", "reflection_end_date"} {
			if row[field] != nil {
				t.Errorf("chunk %d: missing %s replaced with %#v", i, field, row[field])
			}
		}
		if row["ingested_at"] == nil {
			t.Errorf("chunk %d: ingestion timestamp missing", i)
		}
	}
}

func TestRun_ReflectionTitleLabelOnEveryChunk(t *testing.T) {
	t.Setenv("REFLECTION_SPRINT_FROM_MONTH", "")
	file := &domain.File{
		DriveFileID: "synthetic-monthly-id",
		Path:        "202609-3 synthetic-export-id.md",
		MimeType:    "text/markdown",
	}
	// The first heading, rather than the exported filename, supplies the label.
	rows := runReflectionAndCaptureRows(t, file, []byte("# 202610-1\nSummary.\n\n## Actions\nNext step."))
	if len(rows) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(rows))
	}
	for i, row := range rows {
		if row["reflection_month"] != "2026-10" || row["reflection_sequence"] != int32(1) {
			t.Errorf("chunk %d: label = %v/%v, want 2026-10/1", i, row["reflection_month"], row["reflection_sequence"])
		}
		for _, field := range []string{"reflection_start_date", "reflection_end_date"} {
			if row[field] != nil {
				t.Errorf("chunk %d: inferred exact period %s = %v", i, field, row[field])
			}
		}
	}
}

func TestRun_ReflectionSprintOnEveryChunk(t *testing.T) {
	t.Setenv("REFLECTION_SPRINT_FROM_MONTH", "2026-09")
	file := &domain.File{DriveFileID: "synthetic-sprint", Path: "ignored.md", MimeType: "text/markdown"}
	content := []byte("# 202609-4\nSummary.\n\n## Actions\nNext step.")
	rows := runReflectionAndCaptureRows(t, file, content)
	if len(rows) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(rows))
	}
	for i, row := range rows {
		if row["reflection_start_date"] != "2026-09-28" || row["reflection_end_date"] != "2026-10-04" {
			t.Errorf("chunk %d: unexpected period: %v/%v", i, row["reflection_start_date"], row["reflection_end_date"])
		}
		if row["reflection_month"] != "2026-09" {
			t.Errorf("chunk %d: title month changed", i)
		}
	}
	// Reprocessing identical bytes under a corrected policy changes metadata,
	// without changing the content version or retaining the previous period.
	t.Setenv("REFLECTION_SPRINT_FROM_MONTH", "2026-10")
	after := runReflectionAndCaptureRows(t, file, content)
	for i, row := range after {
		if row["content_version"] != rows[i]["content_version"] || row["reflection_start_date"] != nil || row["reflection_end_date"] != nil {
			t.Errorf("chunk %d: metadata refresh failed", i)
		}
	}
}

func TestRun_InvalidSprintConfigurationStopsBeforeReadingFiles(t *testing.T) {
	t.Setenv("REFLECTION_SPRINT_FROM_MONTH", "2026-13")
	// No external dependency may be used when configuration is invalid.
	if err := Run(context.Background(), nil, nil, nil); err == nil {
		t.Fatal("invalid configuration should fail")
	}
}

func TestRun_ReflectionRenamePreservesContentVersionAndUpdatesSource(t *testing.T) {
	content := []byte("# Reflection\nSummary.\n\n## Actions\nNext step.")
	file := &domain.File{
		DriveFileID: "synthetic-reflection-id",
		Path:        "reflection.md",
		Checksum:    "unchanged-checksum",
		MimeType:    "text/markdown",
	}
	before := runReflectionAndCaptureRows(t, file, content)
	renamed := *file
	renamed.Path = "corrected-reflection.md"
	after := runReflectionAndCaptureRows(t, &renamed, content)
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

func runReflectionAndCaptureRows(t *testing.T, file *domain.File, content []byte) []map[string]bigquery.Value {
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
	return captured
}
