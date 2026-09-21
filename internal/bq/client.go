package bq

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
)

type BQClient interface {
	InsertRows(ctx context.Context, table string, rows []map[string]bigquery.Value) error
	ReplaceChunkRows(ctx context.Context, fileID, contentVersion string, rows []map[string]bigquery.Value) error
}

type Client struct {
	bq      *bigquery.Client
	dataset string
}

func NewClient(ctx context.Context) (*Client, error) {
	projectID := os.Getenv("BIGQUERY_PROJECT_ID")
	datasetID := os.Getenv("BIGQUERY_DATASET_ID")

	// ADC (Application Default Credentials) を使用
	// ローカル: gcloud auth application-default login
	// 本番: サービスアカウント
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &Client{bq: client, dataset: datasetID}, nil
}

func (c *Client) Close() {
	c.bq.Close()
}

// InsertRows は NDJSON 形式の Load Job でテーブルへ行を挿入する（無料枠対応）。
func (c *Client) InsertRows(ctx context.Context, table string, rows []map[string]bigquery.Value) error {
	return c.loadRows(ctx, table, rows)
}

// ReplaceChunkRows first loads a complete file version into staging, then
// atomically replaces that file's current rows in chunks. A failed load leaves
// the existing chunks untouched.
func (c *Client) ReplaceChunkRows(ctx context.Context, fileID, contentVersion string, rows []map[string]bigquery.Value) error {
	loadID, err := newLoadID()
	if err != nil {
		return err
	}

	if len(rows) > 0 {
		stagingRows := make([]map[string]bigquery.Value, 0, len(rows))
		for _, row := range rows {
			stagingRow := make(map[string]bigquery.Value, len(row)+3)
			for key, value := range row {
				stagingRow[key] = value
			}
			stagingRow["load_id"] = loadID
			stagingRow["content_version"] = contentVersion
			stagingRow["ingested_at"] = time.Now().UTC()
			stagingRows = append(stagingRows, stagingRow)
		}
		if err := c.loadRows(ctx, "chunks_staging", stagingRows); err != nil {
			return err
		}
	}

	query := c.bq.Query(chunkReplaceQuery(c.bq.Project(), c.dataset))
	query.Parameters = []bigquery.QueryParameter{
		{Name: "fileID", Value: fileID},
		{Name: "loadID", Value: loadID},
	}
	job, err := query.Run(ctx)
	if err != nil {
		return err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}
	return status.Err()
}

func (c *Client) loadRows(ctx context.Context, table string, rows []map[string]bigquery.Value) error {
	var buf bytes.Buffer
	for _, row := range rows {
		line, err := json.Marshal(row)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	src := bigquery.NewReaderSource(&buf)
	src.SourceFormat = bigquery.JSON
	src.AutoDetect = false

	loader := c.bq.Dataset(c.dataset).Table(table).LoaderFrom(src)
	loader.WriteDisposition = bigquery.WriteAppend

	job, err := loader.Run(ctx)
	if err != nil {
		return err
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}

	return status.Err()
}

func newLoadID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate chunk load ID: %w", err)
	}
	return fmt.Sprintf("%x", value), nil
}

func chunkReplaceQuery(projectID, datasetID string) string {
	chunksTable := fmt.Sprintf("`%s.%s.chunks`", projectID, datasetID)
	stagingTable := fmt.Sprintf("`%s.%s.chunks_staging`", projectID, datasetID)
	return fmt.Sprintf(`
BEGIN TRANSACTION;
DELETE FROM %s WHERE file_id = @fileID;
INSERT INTO %s (file_id, chunk_index, content, embedding_status, content_version, ingested_at)
SELECT file_id, chunk_index, content, embedding_status, content_version, ingested_at
FROM %s WHERE load_id = @loadID;
DELETE FROM %s WHERE load_id = @loadID;
COMMIT TRANSACTION;`, chunksTable, chunksTable, stagingTable, stagingTable)
}
