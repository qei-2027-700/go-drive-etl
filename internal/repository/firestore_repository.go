package repository

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"google.golang.org/api/iterator"
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
	DriveFileID string    `firestore:"drive_file_id"`
	Path        string    `firestore:"path"`
	Checksum    string    `firestore:"checksum"`
	MimeType    string    `firestore:"mime_type"`
	SyncStatus  string    `firestore:"sync_status"`
	UpdatedAt   time.Time `firestore:"updated_at,serverTimestamp"`
}

// タスクを登録する（未着手として書き込む）
func (r *FirestoreFileRepository) Upsert(ctx context.Context, f *domain.File) error {
	_, err := r.client.Collection(filesCollection).Doc(f.DriveFileID).Set(ctx, fileDoc{
		DriveFileID: f.DriveFileID,
		Path:        f.Path,
		Checksum:    f.Checksum,
		MimeType:    f.MimeType,
		SyncStatus:  string(f.SyncStatus),
		// time.Now()などは不要
	})

	return err
}

// タスクに完了・失敗ステータスを適用する
func (r *FirestoreFileRepository) UpdateStatus(ctx context.Context, driveFileID string, status domain.SyncStatus) error {
	_, err := r.client.Collection(filesCollection).Doc(driveFileID).Update(ctx, []firestore.Update{
		{Path: "sync_status", Value: string(status)},
		{Path: "updated_at", Value: firestore.ServerTimestamp},
	})
	return err
}

// TODO(#63): sync_status == pending を Where で引き、fileDoc から domain.File に詰め替える。
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
		files = append(files, &domain.File{
			DriveFileID: d.DriveFileID,
			Path:        d.Path,
			Checksum:    d.Checksum,
			MimeType:    d.MimeType,
			SyncStatus:  domain.SyncStatus(d.SyncStatus),
			UpdatedAt:   d.UpdatedAt,
		})
	}

	return files, nil
}
