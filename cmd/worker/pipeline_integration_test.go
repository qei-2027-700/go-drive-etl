//go:build integration

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/firestore"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"github.com/qei-2027-700/go-drive-etl/internal/repository"
	"go.uber.org/mock/gomock"
	driveapi "google.golang.org/api/drive/v3"

	bqMock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	driveMock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
)

// A distinct emulator project isolates this test from repository integration
// tests, which clear the files collection as part of their setup.
const integrationProjectID = "go-drive-etl-integration"

// TestPipelineWithFirestore verifies the complete worker path while keeping
// only the state store real: Drive and BigQuery are mocked, Firestore is the
// emulator supplied by docker-compose locally and the CI service container.
func TestPipelineWithFirestore(t *testing.T) {
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST が未設定のためスキップ")
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, integrationProjectID)
	if err != nil {
		t.Fatalf("Firestore クライアントの初期化: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	fileID := "pipeline-integration-markdown"
	if _, err := client.Collection("files").Doc(fileID).Delete(ctx); err != nil {
		t.Fatalf("既存テストデータの削除: %v", err)
	}
	t.Cleanup(func() { _, _ = client.Collection("files").Doc(fileID).Delete(context.Background()) })

	ctrl := gomock.NewController(t)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	content := []byte("# Overview\nSummary.\n\n## Steps\nDo this.")
	driveClient.EXPECT().ListFiles(gomock.Any(), "folder-123").Return([]*driveapi.File{{
		Id:          fileID,
		Name:        "guide.md",
		Md5Checksum: "checksum-123",
		MimeType:    "text/markdown",
	}}, nil)
	driveClient.EXPECT().DownloadFile(gomock.Any(), fileID).Return(content, nil)

	bqClient.EXPECT().InsertRows(gomock.Any(), "chunks", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, rows []map[string]bigquery.Value) error {
			version := fmt.Sprintf("%x", sha256.Sum256(content))
			if len(rows) != 2 {
				t.Errorf("ChunkRecord count = %d, want 2", len(rows))
			}
			for i, want := range []string{"# Overview\nSummary.", "## Steps\nDo this."} {
				if i >= len(rows) {
					break
				}
				if rows[i]["file_id"] != fileID || rows[i]["chunk_index"] != i || rows[i]["content"] != want ||
					rows[i]["embedding_status"] != "pending" || rows[i]["content_version"] != version || rows[i]["is_deleted"] != false {
					t.Errorf("unexpected chunk row %d: %#v", i, rows[i])
				}
			}
			return nil
		},
	)
	bqClient.EXPECT().InsertRows(gomock.Any(), "drive_files", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, rows []map[string]bigquery.Value) error {
			if len(rows) != 1 || rows[0]["drive_file_id"] != fileID || rows[0]["path"] != "guide.md" ||
				rows[0]["sync_status"] != string(domain.SyncStatusDone) {
				t.Errorf("unexpected drive_files row: %#v", rows)
			}
			return nil
		},
	)

	if err := runPipeline(ctx, repository.NewFirestoreFileRepository(client), driveClient, bqClient, "folder-123"); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}

	doc, err := client.Collection("files").Doc(fileID).Get(ctx)
	if err != nil {
		t.Fatalf("状態管理 DB からの読み出し: %v", err)
	}
	if got := doc.Data()["sync_status"]; got != string(domain.SyncStatusDone) {
		t.Errorf("sync_status = %v, want %q", got, domain.SyncStatusDone)
	}
}
