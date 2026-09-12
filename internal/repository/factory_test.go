package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// New は外部サービスへ即座に接続しない。pgxpool も Firestore クライアントも接続は遅延するため、
// PostgreSQL もエミュレータも GCP 認証情報も無しで全分岐を検証できる。

const (
	testDSN = "postgres://app:password@localhost:5432/app_db?sslmode=disable"

	// 実際に起動している必要はない。クライアント生成が遅延するため接続は発生しない。
	testEmulatorHost = "localhost:8080"
)

// setBackendEnv は New が読む環境変数を毎回すべて明示する。
// 実行環境に残っている値がテスト結果を左右しないようにするため。
func setBackendEnv(t *testing.T, backend, dsn, project, emulatorHost string) {
	t.Helper()

	t.Setenv("STATE_BACKEND", backend)
	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv("GOOGLE_CLOUD_PROJECT", project)
	t.Setenv("FIRESTORE_EMULATOR_HOST", emulatorHost)
}

// STATE_BACKEND が実装を選ぶこと。未設定は Firestore。
func TestNew_SelectsBackend(t *testing.T) {
	tests := []struct {
		name     string
		backend  string
		wantType string
	}{
		{"未設定なら Firestore", "", "*repository.FirestoreFileRepository"},
		{"firestore", "firestore", "*repository.FirestoreFileRepository"},
		{"postgres", "postgres", "*repository.FileRepository"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBackendEnv(t, tt.backend, testDSN, testProjectID, testEmulatorHost)

			repo, closeRepo, err := New(context.Background())
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if closeRepo == nil {
				t.Fatal("後始末関数が nil")
			}
			defer closeRepo()

			if got := fmt.Sprintf("%T", repo); got != tt.wantType {
				t.Errorf("実装: want %s, got %s", tt.wantType, got)
			}
		})
	}
}

// 初期化に失敗したときは repo も後始末関数も返さないこと。
// 呼び出し側が defer closeRepo() を err チェックより先に書いても nil を触らないための契約。
func TestNew_ErrorLeavesNothingToClose(t *testing.T) {
	tests := []struct {
		name         string
		backend      string
		dsn          string
		project      string
		emulatorHost string
	}{
		{"未知のバックエンド", "postgre", testDSN, testProjectID, testEmulatorHost},
		{"DSN が壊れている", "postgres", "これは DSN ではない", testProjectID, testEmulatorHost},
		{"Firestore なのにプロジェクト未設定", "firestore", testDSN, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBackendEnv(t, tt.backend, tt.dsn, tt.project, tt.emulatorHost)

			repo, closeRepo, err := New(context.Background())
			if err == nil {
				t.Fatal("エラーにならなかった")
			}
			if repo != nil {
				t.Errorf("エラー時に repo が返った: %T", repo)
			}
			if closeRepo != nil {
				t.Error("エラー時に後始末関数が返った")
			}
		})
	}
}

// 未知の値が黙って既定の Firestore に落ちないこと。
// タイプミスが本番 Firestore への接続に化けるのを防ぐための分岐なので、
// エラーメッセージが問題の値そのものを示すことまで固定する。
func TestNew_UnknownBackendNamesTheValue(t *testing.T) {
	setBackendEnv(t, "postgre", testDSN, testProjectID, testEmulatorHost)

	_, _, err := New(context.Background())
	if err == nil {
		t.Fatal("未知の値がエラーにならなかった")
	}
	if !strings.Contains(err.Error(), "postgre") {
		t.Errorf("エラーが値を示していない: %v", err)
	}
}
