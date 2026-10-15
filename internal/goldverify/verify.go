// Package goldverify checks report delivery without logging report contents or IDs.
package goldverify

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"

	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/goldreport"
)

var Tables = []string{"mart_ingestion_daily", "mart_file_latest"}

type CSVResult struct {
	Table      string `json:"table"`
	Rows       int    `json:"rows"`
	Columns    int    `json:"columns"`
	Duplicates int    `json:"duplicates"`
	Match      bool   `json:"match"`
	StableID   bool   `json:"stable_id"`
}

// CheckCSV validates exact bytes as well as shape, including header-only exports.
func CheckCSV(table string, want, got []byte, allowEmpty bool) (CSVResult, error) {
	r := CSVResult{Table: table, Match: bytes.Equal(want, got)}
	records, err := csv.NewReader(bytes.NewReader(got)).ReadAll()
	if err != nil || len(records) == 0 {
		return r, fmt.Errorf("invalid CSV")
	}
	r.Rows, r.Columns = len(records)-1, len(records[0])
	columns := 7
	if table == "mart_ingestion_daily" {
		columns = 6
	}
	if r.Columns != columns {
		return r, fmt.Errorf("unexpected column count")
	}
	if !r.Match {
		return r, fmt.Errorf("CSV mismatch")
	}
	if r.Rows == 0 && !allowEmpty {
		return r, fmt.Errorf("unexpected empty CSV")
	}
	return r, nil
}

// CheckDrive returns IDs only in memory, for comparison across exports.
func CheckDrive(ctx context.Context, source goldreport.Source, dest drive.DriveClient, folder string, allowEmpty bool, previous map[string]string) ([]CSVResult, map[string]string, error) {
	files, err := dest.ListFiles(ctx, folder)
	if err != nil {
		return nil, nil, fmt.Errorf("list Drive files: %w", err)
	}
	ids := map[string]string{}
	var results []CSVResult
	for _, table := range Tables {
		r := CSVResult{Table: table}
		count := 0
		for _, f := range files {
			if f.Name == table+".csv" {
				count++
				ids[table] = f.Id
			}
		}
		if count > 1 {
			r.Duplicates = count - 1
		}
		if count != 1 {
			return append(results, r), ids, fmt.Errorf("%s: expected one Drive CSV, found %d", table, count)
		}
		want, err := source.ExportTableCSV(ctx, table)
		if err != nil {
			return append(results, r), ids, fmt.Errorf("export %s: %w", table, err)
		}
		got, err := dest.DownloadFile(ctx, ids[table])
		if err != nil {
			return append(results, r), ids, fmt.Errorf("download %s: %w", table, err)
		}
		r, err = CheckCSV(table, want, got, allowEmpty)
		r.StableID = previous != nil && previous[table] == ids[table]
		results = append(results, r)
		if err != nil {
			return results, ids, err
		}
		if previous != nil && !r.StableID {
			return results, ids, fmt.Errorf("%s: file ID changed", table)
		}
	}
	return results, ids, nil
}
