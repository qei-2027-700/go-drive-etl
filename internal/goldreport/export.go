// Package goldreport provides the production Gold CSV delivery path.
package goldreport

import (
	"context"
	"fmt"
)

type Source interface {
	ExportTableCSV(context.Context, string) ([]byte, error)
}
type Destination interface {
	UpsertFile(context.Context, string, string, string, []byte) error
}

func Export(ctx context.Context, driveClient Destination, bq Source, exportFolderID string) error {
	if exportFolderID == "" {
		return fmt.Errorf("export folder required")
	}
	for _, table := range []string{"mart_ingestion_daily", "mart_file_latest"} {
		contents, err := bq.ExportTableCSV(ctx, table)
		if err != nil {
			return fmt.Errorf("Gold Mart の CSV 抽出に失敗: table=%s: %w", table, err)
		}
		name := table + ".csv"
		if err := driveClient.UpsertFile(ctx, exportFolderID, name, "text/csv", contents); err != nil {
			return fmt.Errorf("Gold Mart CSV の Drive 出力に失敗: file=%s: %w", name, err)
		}
	}
	return nil
}
