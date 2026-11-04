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
