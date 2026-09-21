package bq

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
)

type mockBQClient struct {
	insertRowsCalled bool
	lastTable        string
	lastRows         []map[string]bigquery.Value
	returnErr        error
}

func TestNewLoadID(t *testing.T) {
	first, err := newLoadID()
	if err != nil {
		t.Fatalf("newLoadID() error = %v", err)
	}
	second, err := newLoadID()
	if err != nil {
		t.Fatalf("newLoadID() error = %v", err)
	}
	if len(first) != 32 {
		t.Errorf("load ID length = %d, want 32", len(first))
	}
	if first == second {
		t.Error("two generated load IDs must differ")
	}
}

func TestChunkReplaceQuery(t *testing.T) {
	query := chunkReplaceQuery("project", "dataset")
	for _, want := range []string{
		"BEGIN TRANSACTION",
		"DELETE FROM `project.dataset.chunks` WHERE file_id = @fileID",
		"FROM `project.dataset.chunks_staging` WHERE load_id = @loadID",
		"COMMIT TRANSACTION",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query does not contain %q:\n%s", want, query)
		}
	}
}

func (m *mockBQClient) InsertRows(ctx context.Context, table string, rows []map[string]bigquery.Value) error {
	m.insertRowsCalled = true
	m.lastTable = table
	m.lastRows = rows
	return m.returnErr
}

func TestMockBQClient_InsertRows(t *testing.T) {
	mock := &mockBQClient{}

	rows := []map[string]bigquery.Value{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	err := mock.InsertRows(context.Background(), "my_table", rows)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mock.insertRowsCalled {
		t.Error("InsertRows が呼ばれていない")
	}
	if mock.lastTable != "my_table" {
		t.Errorf("expected table 'my_table', got %s", mock.lastTable)
	}
}

func TestMockBQClient_InsertRows_Error(t *testing.T) {
	mock := &mockBQClient{returnErr: fmt.Errorf("BQ error")}

	err := mock.InsertRows(context.Background(), "my_table", nil)

	if err == nil {
		t.Error("エラーが返ってこなかった")
	}
}
