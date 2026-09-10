package repository

import (
	"context"
	"os"
	"testing"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/qei-2027-700/go-drive-etl/internal/domain"
)

// エミュレータ内でのみ有効な識別子。実在の GCP プロジェクトである必要はない。
const testProjectID = "go-drive-etl-test"

// newTestRepo は Firestore エミュレータに接続したリポジトリを返す。
// エミュレータが無い環境（CI など）ではテストをスキップする。
//
//	docker compose up -d firestore
//	FIRESTORE_EMULATOR_HOST=localhost:8080 go test ./internal/repository/
func newTestRepo(t *testing.T) *FirestoreFileRepository {
	t.Helper()

	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST が未設定のためスキップ")
	}

	client, err := firestore.NewClient(context.Background(), testProjectID)
	if err != nil {
		t.Fatalf("Firestore クライアントの初期化に失敗: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	clearFiles(t, client)

	return NewFirestoreFileRepository(client)
}

// clearFiles は files コレクションを空にする。テストは同じコレクションを共有するため、
// 前のテストが残したドキュメントで件数の検証が壊れないようにする。
func clearFiles(t *testing.T, client *firestore.Client) {
	t.Helper()

	ctx := context.Background()
	iter := client.Collection(filesCollection).Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return
		}
		if err != nil {
			t.Fatalf("既存ドキュメントの走査に失敗: %v", err)
		}
		if _, err := doc.Ref.Delete(ctx); err != nil {
			t.Fatalf("既存ドキュメントの削除に失敗: %v", err)
		}
	}
}

func testFile() *domain.File {
	return &domain.File{
		DriveFileID: "test-drive-id",
		Path:        "memo.md",
		Checksum:    "abc123",
		MimeType:    "text/markdown",
		SyncStatus:  domain.SyncStatusPending,
	}
}

// Upsert したファイルが ListPending で取得でき、全フィールドが往復すること。
func TestFirestore_UpsertAndListPending(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	want := testFile()
	if err := repo.Upsert(ctx, want); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := repo.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("件数: want 1, got %d", len(got))
	}

	f := got[0]
	if f.DriveFileID != want.DriveFileID {
		t.Errorf("DriveFileID: want %q, got %q", want.DriveFileID, f.DriveFileID)
	}
	if f.Path != want.Path {
		t.Errorf("Path: want %q, got %q", want.Path, f.Path)
	}
	if f.Checksum != want.Checksum {
		t.Errorf("Checksum: want %q, got %q", want.Checksum, f.Checksum)
	}
	if f.MimeType != want.MimeType {
		t.Errorf("MimeType: want %q, got %q", want.MimeType, f.MimeType)
	}
	if f.SyncStatus != domain.SyncStatusPending {
		t.Errorf("SyncStatus: want %q, got %q", domain.SyncStatusPending, f.SyncStatus)
	}
	// serverTimestamp タグが効いていればサーバ側の時刻が入る。
	if f.UpdatedAt.IsZero() {
		t.Error("UpdatedAt がゼロ値。serverTimestamp が機能していない")
	}
}

// pending 以外になったファイルは ListPending から外れること。
func TestFirestore_UpdateStatusRemovesFromPending(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	f := testFile()
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := repo.UpdateStatus(ctx, f.DriveFileID, domain.SyncStatusDone); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := repo.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("done にしたファイルが pending に残っている: got %d 件", len(got))
	}
}

// UpdateStatus は sync_status 以外のフィールドを消さないこと（Set ではなく Update を使う理由）。
func TestFirestore_UpdateStatusPreservesOtherFields(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	f := testFile()
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := repo.UpdateStatus(ctx, f.DriveFileID, domain.SyncStatusFailed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// failed は pending ではないので、確認のため pending に戻して読み直す。
	if err := repo.UpdateStatus(ctx, f.DriveFileID, domain.SyncStatusPending); err != nil {
		t.Fatalf("UpdateStatus(pending): %v", err)
	}

	got, err := repo.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("件数: want 1, got %d", len(got))
	}
	if got[0].Path != f.Path || got[0].Checksum != f.Checksum || got[0].MimeType != f.MimeType {
		t.Errorf("UpdateStatus が他フィールドを壊した: %+v", *got[0])
	}
}

// 同じ drive_file_id への Upsert がドキュメントを増やさず上書きすること。
// ドキュメント ID に drive_file_id を使う設計の核心。
func TestFirestore_UpsertIsIdempotent(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	f := testFile()
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert(1回目): %v", err)
	}

	f.Checksum = "xyz789"
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert(2回目): %v", err)
	}

	got, err := repo.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("同じ ID の Upsert でドキュメントが増えた: want 1, got %d", len(got))
	}
	if got[0].Checksum != "xyz789" {
		t.Errorf("Checksum が更新されていない: want %q, got %q", "xyz789", got[0].Checksum)
	}
}

// 存在しないドキュメントへの UpdateStatus は NotFound を返すこと。
// Postgres の UPDATE ... WHERE は no-op だったため、移行で変わった挙動を明示的に固定する。
func TestFirestore_UpdateStatusOnMissingDocument(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, "no-such-id", domain.SyncStatusFailed)
	if err == nil {
		t.Fatal("存在しない ID でエラーにならなかった")
	}
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("エラーコード: want %v, got %v (%v)", codes.NotFound, got, err)
	}
}
