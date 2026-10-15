package goldverify

import (
	"context"
	"errors"
	"strings"
	"testing"

	bqmock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	drivemock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
	"go.uber.org/mock/gomock"
	driveapi "google.golang.org/api/drive/v3"
)

var fixture = []byte("a,b,c,d,e,f,g\n1,2,3,4,5,6,7\n")

func TestCSVFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		want, got []byte
		allow, ok bool
	}{
		{"exact", fixture, fixture, false, true},
		{"stale", fixture, []byte("a,b,c,d,e,f,g\n7,6,5,4,3,2,1\n"), false, false},
		{"empty rejected", fixture[:14], fixture[:14], false, false},
		{"empty allowed", fixture[:14], fixture[:14], true, true},
		{"wrong shape", []byte("a,b\n1,2\n"), []byte("a,b\n1,2\n"), false, false},
		{"malformed", fixture, []byte("\""), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckCSV("test", tc.want, tc.got, tc.allow)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestDriveChecks(t *testing.T) {
	for _, mode := range []string{"ok", "duplicate", "missing", "auth", "download", "changed", "stale"} {
		t.Run(mode, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			src := bqmock.NewMockBQClient(ctrl)
			dest := drivemock.NewMockDriveClient(ctrl)
			files := []*driveapi.File{{Name: Tables[0] + ".csv", Id: "synthetic-a"}, {Name: Tables[1] + ".csv", Id: "synthetic-b"}}
			previous := map[string]string{Tables[0]: "synthetic-a", Tables[1]: "synthetic-b"}
			if mode == "changed" {
				previous[Tables[0]] = "synthetic-old"
			}
			if mode == "duplicate" {
				files = append(files, files[0])
			}
			if mode == "missing" {
				files = nil
			}
			var listErr error
			if mode == "auth" {
				listErr = errors.New("expired authentication")
			}
			dest.EXPECT().ListFiles(gomock.Any(), "folder").Return(files, listErr)
			if mode != "auth" && mode != "duplicate" && mode != "missing" {
				limit := 2
				if mode != "ok" {
					limit = 1
				}
				for i := 0; i < limit; i++ {
					data := fixture
					if i == 0 {
						data = []byte("a,b,c,d,e,f\n1,2,3,4,5,6\n")
					}
					src.EXPECT().ExportTableCSV(gomock.Any(), Tables[i]).Return(data, nil)
					got := data
					var err error
					if mode == "download" {
						err = errors.New("download denied")
					}
					if mode == "stale" {
						got = []byte("a,b,c,d,e,f,g\n7,6,5,4,3,2,1\n")
					}
					dest.EXPECT().DownloadFile(gomock.Any(), files[i].Id).Return(got, err)
				}
			}
			results, _, err := CheckDrive(context.Background(), src, dest, "folder", false, previous)
			if (err == nil) != (mode == "ok") {
				t.Fatalf("err=%v", err)
			}
			if mode == "duplicate" && results[0].Duplicates != 1 {
				t.Fatal("missing duplicate count")
			}
		})
	}
}

func TestSQLConfiguration(t *testing.T) {
	if _, err := ComparisonSQL("test-project", "etl_silver", "etl_gold"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"bad`; DROP TABLE x", "a.b", ""} {
		if _, err := ComparisonSQL("test-project", v, "etl_gold"); err == nil {
			t.Fatal("unsafe dataset accepted")
		}
	}
	sql, _ := ComparisonSQL("test-project", "etl_silver", "etl_gold")
	if strings.Count(sql, "EXCEPT DISTINCT") != 4 || !strings.Contains(sql, "`test-project.etl_silver.silver_chunks`") {
		t.Fatal("incomplete comparison")
	}
}

func TestGoldResults(t *testing.T) {
	base := map[string]int64{"silver_chunks": 4, "expected_daily": 1, "actual_daily": 1, "expected_latest": 2, "actual_latest": 2}
	if !GoldMatches(base, false) {
		t.Fatal("valid result rejected")
	}
	for _, key := range []string{"daily_missing", "daily_extra", "latest_missing", "latest_extra", "actual_daily", "actual_latest"} {
		c := map[string]int64{}
		for k, v := range base {
			c[k] = v
		}
		c[key]++
		if GoldMatches(c, false) {
			t.Fatalf("missed stale/different/duplicate rows: %s", key)
		}
	}
	if GoldMatches(map[string]int64{}, false) {
		t.Fatal("unexpected empty accepted")
	}
	if !GoldMatches(map[string]int64{}, true) {
		t.Fatal("intentional empty rejected")
	}
	c := map[string]int64{"silver_chunks": 4, "expected_latest": 2, "expected_daily": 1}
	if GoldMatches(c, true) {
		t.Fatal("missing Gold accepted with allow-empty")
	}
}
