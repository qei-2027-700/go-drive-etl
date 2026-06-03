package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qei-2027-700/go-drive-etl/internal/domain"
)

type FileRepository struct {
	db *pgxpool.Pool
}

type FileRepo interface {
	ListPending(ctx context.Context) ([]*domain.File, error)
	UpdateStatus(ctx context.Context, fileID int64, status domain.SyncStatus) error
	Upsert(ctx context.Context, f *domain.File) error
}

func NewFileRepository(db *pgxpool.Pool) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Upsert(ctx context.Context, f *domain.File) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO files (drive_file_id, path, checksum, mime_type, sync_status, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (drive_file_id)
		DO UPDATE SET
				checksum    = EXCLUDED.checksum,
				sync_status = EXCLUDED.sync_status,
				updated_at  = NOW()
		`, f.DriveFileID, f.Path, f.Checksum, f.MimeType, f.SyncStatus)

	return err
}

func (r *FileRepository) UpdateStatus(ctx context.Context, fileID int64, status domain.SyncStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE files SET sync_status = $1, updated_at = NOW() WHERE id = $2`,
		status, fileID,
	)
	return err
}

func (r *FileRepository) ListPending(ctx context.Context) ([]*domain.File, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, drive_file_id, path, checksum, mime_type, sync_status, updated_at
		FROM files WHERE sync_status = 'pending'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*domain.File
	for rows.Next() {
		f := &domain.File{}
		if err := rows.Scan(&f.ID, &f.DriveFileID, &f.Path, &f.Checksum, &f.MimeType, &f.SyncStatus, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, rows.Err()
}
