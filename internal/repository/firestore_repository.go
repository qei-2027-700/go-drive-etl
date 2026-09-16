package repository

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const filesCollection = "files"

type FirestoreFileRepository struct {
	client *firestore.Client
}

// コンパイル時にインターフェースを満たしているか検査する。
var _ FileRepo = (*FirestoreFileRepository)(nil)

func NewFirestoreFileRepository(client *firestore.Client) *FirestoreFileRepository {
	return &FirestoreFileRepository{client: client}
}

// fileDoc は Firestore 上のドキュメント表現。
// domain.File を Firestore のタグで汚さないため repository 層に閉じ込める。
type fileDoc struct {
	// ドキュメント ID と同じ値を冗長に持つ。読み出しは doc.Ref.ID を使うため
	// コード側では参照しないが、コンソールやエクスポートで ID が見えるように保存する。
	DriveFileID string    `firestore:"drive_file_id"`
	Path        string    `firestore:"path"`
	Checksum    string    `firestore:"checksum"`
	MimeType    string    `firestore:"mime_type"`
	SyncStatus  string    `firestore:"sync_status"`
	UpdatedAt   time.Time `firestore:"updated_at,serverTimestamp"`
}

// Upsert は新規ファイルを pending として登録する。既存ファイルは checksum が変わった
// 場合だけ pending に戻す。md5Checksum が空の Google Workspace ファイルは変更を比較
// できないため、常に再処理対象として更新する。
func (r *FirestoreFileRepository) Upsert(ctx context.Context, f *domain.File) error {
	docRef := r.client.Collection(filesCollection).Doc(f.DriveFileID)
	return r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) != codes.NotFound {
				return err
			}
			return tx.Set(docRef, newFileDoc(f))
		}

		var existing fileDoc
		if err := doc.DataTo(&existing); err != nil {
			return err
		}
		if f.Checksum != "" && existing.Checksum == f.Checksum {
			return nil
		}
		return tx.Set(docRef, newFileDoc(f))
	})
}

func newFileDoc(f *domain.File) fileDoc {
	return fileDoc{
		DriveFileID: f.DriveFileID,
		Path:        f.Path,
		Checksum:    f.Checksum,
		MimeType:    f.MimeType,
		SyncStatus:  string(domain.SyncStatusPending),
		// time.Now()などは不要
	}
}

// タスクに完了・失敗ステータスを適用する
func (r *FirestoreFileRepository) UpdateStatus(ctx context.Context, driveFileID string, status domain.SyncStatus) error {
	_, err := r.client.Collection(filesCollection).Doc(driveFileID).Update(ctx, []firestore.Update{
		{Path: "sync_status", Value: string(status)},
		{Path: "updated_at", Value: firestore.ServerTimestamp},
	})
	return err
}

// 未着手のタスクを全部持ってくる
func (r *FirestoreFileRepository) ListPending(ctx context.Context) ([]*domain.File, error) {

	// クエリを組み立てる
	iter := r.client.Collection(filesCollection).
		Where("sync_status", "==", string(domain.SyncStatusPending)).
		Documents(ctx)
	defer iter.Stop()

	var files []*domain.File

	// ループで1件ずつ取り出す
	for {
		// Firestoreから1件、型なしの状態で受け取る
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var d fileDoc
		// タグを頼りに d へ流し込む（型がつく）
		if err := doc.DataTo(&d); err != nil {
			return nil, err
		}

		// domainパッケージを Firestore に依存させないため、 d の内容を、domain.File へ詰め替える
		// ID はドキュメント ID を正とする（フィールド側が欠けていても壊れない）
		files = append(files, &domain.File{
			DriveFileID: doc.Ref.ID,
			Path:        d.Path,
			Checksum:    d.Checksum,
			MimeType:    d.MimeType,
			SyncStatus:  domain.SyncStatus(d.SyncStatus),
			UpdatedAt:   d.UpdatedAt,
		})
	}

	return files, nil
}
