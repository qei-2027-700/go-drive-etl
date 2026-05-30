package domain

import "time"

type SyncStatus string

const (
	SyncStatusPending    SyncStatus = "pending"
	SyncStatusProcessing SyncStatus = "processing"
	SyncStatusDone       SyncStatus = "done"
	SyncStatusFailed     SyncStatus = "failed"
)

type File struct {
	ID          int64
	DriveFileID string
	Path        string
	Checksum    string
	MimeType    string
	SyncStatus  SyncStatus
	UpdatedAt   time.Time
}

type Chunk struct {
	ID              int64
	FileID          int64
	ChunkIndex      int
	Content         string
	EmbeddingStatus string
}

type Job struct {
	ID         int64
	Status     string
	RetryCount int
	StartedAt  time.Time
}
