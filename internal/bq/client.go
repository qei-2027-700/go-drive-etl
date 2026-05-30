package bq

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

	"cloud.google.com/go/bigquery"
)

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
