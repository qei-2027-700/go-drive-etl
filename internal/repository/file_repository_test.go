package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qei-2027-700/go-drive-etl/internal/domain"
)

// newTestPostgresRepo は POSTGRES_TEST_DSN が指す PostgreSQL に接続したリポジトリを返す。
// 未設定の環境（CI など）ではテストをスキップする。
//
// **このヘルパは files テーブルを空にする。** 接続先を取り違えないよう、アプリが読む
// POSTGRES_DSN ではなく専用の変数を使い、明示的に指定したときだけ動くようにしている。
//
//	docker compose up -d postgres
//	docker compose exec -T postgres psql -U app -d app_db < migrations/001_init.sql
//	POSTGRES_TEST_DSN=postgres://app:password@localhost:5432/app_db?sslmode=disable go test ./internal/repository/
func newTestPostgresRepo(t *testing.T) *FileRepository {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN が未設定のためスキップ")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("接続プールの作成に失敗: %v", err)
	}
	t.Cleanup(pool.Close)

	// テストは同じテーブルを共有するため、前のテストが残した行で件数の検証が壊れないようにする。
	// chunks が files を参照しているので CASCADE が要る。
	if _, err := pool.Exec(context.Background(), "TRUNCATE files RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("既存行の削除に失敗: %v", err)
	}

	return NewFileRepository(pool)
}

// Upsert したファイルが ListPending で取得でき、全フィールドが往復すること。
func TestPostgres_UpsertAndListPending(t *testing.T) {
	repo := newTestPostgresRepo(t)
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
	if f.UpdatedAt.IsZero() {
		t.Error("UpdatedAt がゼロ値")
	}
}

// pending 以外になったファイルは ListPending から外れること。
func TestPostgres_UpdateStatusRemovesFromPending(t *testing.T) {
	repo := newTestPostgresRepo(t)
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

// UpdateStatus は sync_status 以外の列を壊さないこと。
func TestPostgres_UpdateStatusPreservesOtherFields(t *testing.T) {
	repo := newTestPostgresRepo(t)
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
		t.Errorf("UpdateStatus が他の列を壊した: %+v", *got[0])
	}
}

// 同じ drive_file_id への Upsert が行を増やさず、Firestore の Set と同じ範囲を更新すること。
// ON CONFLICT DO UPDATE に path と mime_type を含めているのはこの一致のため。
func TestPostgres_UpsertIsIdempotent(t *testing.T) {
	repo := newTestPostgresRepo(t)
	ctx := context.Background()

	f := testFile()
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert(1回目): %v", err)
	}

	f.Path = "renamed.md"
	f.Checksum = "xyz789"
	f.MimeType = "text/plain"
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert(2回目): %v", err)
	}

	got, err := repo.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("同じ ID の Upsert で行が増えた: want 1, got %d", len(got))
	}
	if got[0].Path != f.Path {
		t.Errorf("Path が更新されていない: want %q, got %q", f.Path, got[0].Path)
	}
	if got[0].Checksum != f.Checksum {
		t.Errorf("Checksum が更新されていない: want %q, got %q", f.Checksum, got[0].Checksum)
	}
	if got[0].MimeType != f.MimeType {
		t.Errorf("MimeType が更新されていない: want %q, got %q", f.MimeType, got[0].MimeType)
	}
}

// 存在しない行への UpdateStatus は no-op でエラーにならないこと。
// Firestore の Update は NotFound を返す。移行で変わった挙動を両側から固定する。
func TestPostgres_UpdateStatusOnMissingRow(t *testing.T) {
	repo := newTestPostgresRepo(t)
	ctx := context.Background()

	if err := repo.UpdateStatus(ctx, "no-such-id", domain.SyncStatusFailed); err != nil {
		t.Errorf("存在しない ID でエラーになった: %v", err)
	}
}
