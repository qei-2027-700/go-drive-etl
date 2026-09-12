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
	DriveFileID string
	Path        string
	Checksum    string
	MimeType    string
	SyncStatus  SyncStatus
	UpdatedAt   time.Time
}

// TODO: Issue #63 で File.ID を廃止したため、FileID は Phase 2 で DriveFileID string に置き換える
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
