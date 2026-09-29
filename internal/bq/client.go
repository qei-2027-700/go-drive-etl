package bq

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

type BQClient interface {
	InsertRows(ctx context.Context, table string, rows []map[string]bigquery.Value) error
	ExportTableCSV(ctx context.Context, table string) ([]byte, error)
}

var tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Client struct {
	bq            *bigquery.Client
	dataset       string
	exportDataset string
}

func NewClient(ctx context.Context) (*Client, error) {
	projectID := os.Getenv("BIGQUERY_PROJECT_ID")
	datasetID := os.Getenv("BIGQUERY_DATASET_ID")
	exportDatasetID := os.Getenv("BIGQUERY_GOLD_DATASET_ID")
	if exportDatasetID == "" {
		exportDatasetID = "etl_gold"
	}

	// ADC (Application Default Credentials) を使用
	// ローカル: gcloud auth application-default login
	// 本番: サービスアカウント
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &Client{bq: client, dataset: datasetID, exportDataset: exportDatasetID}, nil
}

func (c *Client) Close() {
	c.bq.Close()
}

// InsertRows は NDJSON 形式の Load Job でテーブルへ行を挿入する（無料枠対応）。
func (c *Client) InsertRows(ctx context.Context, table string, rows []map[string]bigquery.Value) error {
	return c.loadRows(ctx, table, rows)
}

// ExportTableCSV reads a BigQuery table and renders its schema and rows as CSV.
// Table names are validated before they become part of the query identifier.
func (c *Client) ExportTableCSV(ctx context.Context, table string) ([]byte, error) {
	if !tableNamePattern.MatchString(table) {
		return nil, fmt.Errorf("invalid table name %q", table)
	}

	query := c.bq.Query(fmt.Sprintf("SELECT * FROM `%s.%s.%s` ORDER BY 1", c.bq.Project(), c.exportDataset, table))
	iter, err := query.Read(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	row := map[string]bigquery.Value{}
	firstErr := iter.Next(&row)
	if firstErr != nil && firstErr != iterator.Done {
		return nil, firstErr
	}
	// Query.Read can defer schema population until the first Next call.
	header := make([]string, len(iter.Schema))
	for i, field := range iter.Schema {
		header[i] = field.Name
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	writeRow := func(row map[string]bigquery.Value) error {
		record := make([]string, len(iter.Schema))
		for i, field := range iter.Schema {
			if value := row[field.Name]; value != nil {
				record[i] = fmt.Sprint(value)
			}
		}
		return w.Write(record)
	}
	if firstErr != iterator.Done {
		if err := writeRow(row); err != nil {
			return nil, err
		}
	}
	for {
		row = map[string]bigquery.Value{}
		err := iter.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := writeRow(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
