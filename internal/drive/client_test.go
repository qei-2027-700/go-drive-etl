package drive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	svc, err := driveapi.NewService(context.Background(), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.BasePath = server.URL + "/drive/v3/"

	return &Client{svc: svc}
}

func TestClientListFiles(t *testing.T) {
	var requests []string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/drive/v3/files" {
			t.Errorf("path: got %q, want %q", r.URL.Path, "/drive/v3/files")
		}
		requests = append(requests, r.URL.RawQuery)

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "next-page" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]string{{"id": "file-2", "name": "second.csv"}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"files":         []map[string]string{{"id": "file-1", "name": "first.csv"}},
			"nextPageToken": "next-page",
		})
	}))

	files, err := client.ListFiles(context.Background(), "folder-123")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("file count: got %d, want 2", len(files))
	}
	if files[0].Id != "file-1" || files[1].Id != "file-2" {
		t.Errorf("file IDs: got %q, %q", files[0].Id, files[1].Id)
	}
	if len(requests) != 2 {
		t.Fatalf("request count: got %d, want 2", len(requests))
	}

	query, err := url.ParseQuery(requests[0])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if got, want := query.Get("q"), "'folder-123' in parents and trashed = false"; got != want {
		t.Errorf("q: got %q, want %q", got, want)
	}
	if got := query.Get("pageToken"); got != "" {
		t.Errorf("first page token: got %q, want empty", got)
	}

	query, err = url.ParseQuery(requests[1])
	if err != nil {
		t.Fatalf("parse second query: %v", err)
	}
	if got, want := query.Get("pageToken"), "next-page"; got != want {
		t.Errorf("second page token: got %q, want %q", got, want)
	}
}

func TestClientListFilesWithoutFolder(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("q"), "trashed = false"; got != want {
			t.Errorf("q: got %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
	}))

	files, err := client.ListFiles(context.Background(), "")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("file count: got %d, want 0", len(files))
	}
}

func TestClientDownloadFile(t *testing.T) {
	want := []byte("file contents\n")
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/drive/v3/files/file-123" {
			t.Errorf("path: got %q, want %q", r.URL.Path, "/drive/v3/files/file-123")
		}
		if got, want := r.URL.Query().Get("alt"), "media"; got != want {
			t.Errorf("alt: got %q, want %q", got, want)
		}
		_, _ = w.Write(want)
	}))

	got, err := client.DownloadFile(context.Background(), "file-123")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("contents: got %q, want %q", got, want)
	}
}
