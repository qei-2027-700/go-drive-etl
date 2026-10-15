package repository

import (
	"context"
	"testing"

	"github.com/qei-2027-700/go-drive-etl/internal/domain"
)

// Exercise the same metadata-change contract against both state backends.
// These tests use the existing opt-in database helpers; no production DB is used.
func TestReflectionMetadataUpsert(t *testing.T) {
	backends := []struct {
		name string
		new  func(*testing.T) FileRepo
	}{
		{"Firestore", func(t *testing.T) FileRepo { return newTestRepo(t) }},
		{"Postgres", func(t *testing.T) FileRepo { return newTestPostgresRepo(t) }},
	}
	cases := []struct {
		name        string
		status      domain.SyncStatus
		newPath     string
		wantPending bool
	}{
		{"unchanged completed document", domain.SyncStatusDone, "memo.md", false},
		{"renamed completed document", domain.SyncStatusDone, "renamed.md", true},
		{"date corrected in filename", domain.SyncStatusDone, "2026-10-06_reflection.md", true},
		{"renamed pending document", domain.SyncStatusPending, "renamed.md", true},
		{"unchanged failed document", domain.SyncStatusFailed, "memo.md", true},
	}
	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					repo := backend.new(t)
					ctx := context.Background()
					original := testFile()
					if tc.name == "date corrected in filename" {
						original.Path = "2026-10-05_reflection.md"
					}
					if err := repo.Upsert(ctx, original); err != nil {
						t.Fatalf("initial Upsert: %v", err)
					}
					if err := repo.UpdateStatus(ctx, original.DriveFileID, tc.status); err != nil {
						t.Fatalf("UpdateStatus: %v", err)
					}
					updated := *original
					updated.Path = tc.newPath
					if err := repo.Upsert(ctx, &updated); err != nil {
						t.Fatalf("metadata Upsert: %v", err)
					}
					pending, err := repo.ListPending(ctx)
					if err != nil {
						t.Fatalf("ListPending: %v", err)
					}
					if !tc.wantPending {
						if len(pending) != 0 {
							t.Fatalf("unchanged document queued again: %+v", pending)
						}
						return
					}
					if len(pending) != 1 {
						t.Fatalf("pending count = %d, want 1", len(pending))
					}
					got := pending[0]
					if got.DriveFileID != updated.DriveFileID || got.Path != updated.Path ||
						got.Checksum != original.Checksum || got.MimeType != original.MimeType ||
						got.SyncStatus != domain.SyncStatusPending {
						t.Errorf("updated metadata not preserved: got %+v, want %+v with pending status", *got, updated)
					}
					// Rediscovering the same pending document must not duplicate it
					// or restore the old name.
					if err := repo.Upsert(ctx, &updated); err != nil {
						t.Fatalf("repeated Upsert: %v", err)
					}
					pending, err = repo.ListPending(ctx)
					if err != nil {
						t.Fatalf("repeated ListPending: %v", err)
					}
					if len(pending) != 1 || pending[0].Path != updated.Path {
						t.Errorf("repeated discovery changed pending metadata: %+v", pending)
					}
				})
			}
		})
	}
}
