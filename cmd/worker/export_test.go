package main

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	bqMock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	driveMock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
)

func TestExportGoldReports(t *testing.T) {
	ctrl := gomock.NewController(t)
	bqClient := bqMock.NewMockBQClient(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)

	bqClient.EXPECT().ExportTableCSV(gomock.Any(), "mart_ingestion_daily").Return([]byte("ingested_date,file_count\n2026-01-01,1\n"), nil)
	driveClient.EXPECT().UpsertFile(gomock.Any(), "exports", "mart_ingestion_daily.csv", "text/csv", []byte("ingested_date,file_count\n2026-01-01,1\n")).Return(nil)
	bqClient.EXPECT().ExportTableCSV(gomock.Any(), "mart_file_latest").Return([]byte("file_id,chunk_count\na,2\n"), nil)
	driveClient.EXPECT().UpsertFile(gomock.Any(), "exports", "mart_file_latest.csv", "text/csv", []byte("file_id,chunk_count\na,2\n")).Return(nil)

	if err := exportGoldReports(context.Background(), driveClient, bqClient, "exports"); err != nil {
		t.Fatalf("exportGoldReports: %v", err)
	}
}

func TestExportGoldReportsSkipsWithoutFolder(t *testing.T) {
	ctrl := gomock.NewController(t)
	if err := exportGoldReports(context.Background(), driveMock.NewMockDriveClient(ctrl), bqMock.NewMockBQClient(ctrl), ""); err != nil {
		t.Fatalf("exportGoldReports: %v", err)
	}
}
