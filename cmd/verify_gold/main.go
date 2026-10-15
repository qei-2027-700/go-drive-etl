// Command verify_gold performs explicitly requested live checks. go test never runs it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/joho/godotenv"
	bqclient "github.com/qei-2027-700/go-drive-etl/internal/bq"
	"github.com/qei-2027-700/go-drive-etl/internal/drive"
	"github.com/qei-2027-700/go-drive-etl/internal/goldreport"
	"github.com/qei-2027-700/go-drive-etl/internal/goldverify"
)

type stage struct {
	ErrorCode string                 `json:"error_code,omitempty"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Counts    map[string]int64       `json:"counts,omitempty"`
	CSV       []goldverify.CSVResult `json:"csv,omitempty"`
}
type report struct {
	Started         string  `json:"started"`
	Finished        string  `json:"finished"`
	Status          string  `json:"status"`
	Stages          []stage `json:"stages"`
	DashboardURL    string  `json:"dashboard_url"`
	DashboardReason string  `json:"dashboard_reason"`
}

func main() { os.Exit(run()) }
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run() int {
	mutate := flag.Bool("refresh-and-export", false, "rebuild Gold and export twice (cloud writes)")
	empty := flag.Bool("allow-empty", false, "accept empty Silver and header-only CSVs")
	flag.Parse()
	// Parse dotenv as data; never execute it as shell code. Existing env wins.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Println("invalid .env")
		return 1
	}
	dir, err := os.MkdirTemp("", "gold-verification-")
	if err != nil {
		fmt.Println("cannot create private log directory")
		return 1
	}
	r := newReport(*mutate)
	r.Started = time.Now().UTC().Format(time.RFC3339)
	r.DashboardURL = os.Getenv("GOLD_DASHBOARD_URL")
	r.DashboardReason = "Browser rendering has not been checked. Open the report with a signed-in browser and compare the dashboard_files/dashboard_pending counts with all-date scorecards; check daily chart and latest-file table as described in docs/looker-studio.md."
	if r.DashboardURL == "" {
		if data, err := os.ReadFile("docs/looker-studio.md"); err == nil {
			r.DashboardURL = regexp.MustCompile(`https://datastudio\.google\.com/[^\s]+`).FindString(string(data))
		}
	}
	finish := func(err error) int {
		code := 2
		if err != nil {
			code = 1
			// Detailed API errors may contain identifiers or credentials: keep them private.
			if writeErr := os.WriteFile(filepath.Join(dir, "error.txt"), []byte(err.Error()), 0600); writeErr != nil {
				fmt.Println("cannot save private error log")
			}
		}
		r.Finished = time.Now().UTC().Format(time.RFC3339)
		data, _ := json.MarshalIndent(r, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "summary.json"), data, 0600); err != nil {
			fmt.Println("cannot save summary")
			return 1
		}
		fmt.Printf("%s\nlog: %s\n", data, filepath.Join(dir, "summary.json"))
		return code
	}
	project := os.Getenv("BIGQUERY_PROJECT_ID")
	silver := value("BQ_SILVER_DATASET_ID", "etl_silver")
	gold := value("BIGQUERY_GOLD_DATASET_ID", "etl_gold")
	location := value("BQ_LOCATION", "asia-northeast1")
	folder := os.Getenv("DRIVE_EXPORT_FOLDER_ID")
	sql, err := goldverify.ComparisonSQL(project, silver, gold)
	if err != nil {
		return finish(r.fail(configurationStage, err))
	}
	if folder == "" {
		return finish(r.fail(configurationStage, fmt.Errorf("DRIVE_EXPORT_FOLDER_ID required")))
	}
	r.Stages[configurationStage].Status = "passed"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	// Lazily construct CSV clients only after Gold comparison succeeds.
	var src *bqclient.Client
	var dest *drive.Client
	defer func() {
		if src != nil {
			src.Close()
		}
	}()
	csvClients := func(ctx context.Context) error {
		if src == nil {
			var err error
			src, err = bqclient.NewClient(ctx)
			if err != nil {
				return err
			}
		}
		if dest == nil {
			var err error
			dest, err = drive.NewClient(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	ops := verificationOps{
		refresh: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, "bash", "scripts/refresh_gold_marts.sh")
			cmd.Env = append(os.Environ(), "BQ_PROJECT_ID="+project, "BQ_GOLD_DATASET_ID="+gold, "BQ_SILVER_DATASET_ID="+silver, "BQ_LOCATION="+location)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, output)
			}
			return nil
		},
		compare: func(ctx context.Context) (map[string]int64, error) {
			client, err := bigquery.NewClient(ctx, project)
			if err != nil {
				return nil, err
			}
			defer client.Close()
			query := client.Query(sql)
			query.Location = location
			iter, err := query.Read(ctx)
			if err != nil {
				return nil, err
			}
			row := map[string]bigquery.Value{}
			if err = iter.Next(&row); err != nil {
				return nil, err
			}
			counts := map[string]int64{}
			for k, v := range row {
				n, ok := v.(int64)
				if !ok {
					return nil, fmt.Errorf("unexpected count type")
				}
				counts[k] = n
			}
			return counts, nil
		},
		export: func(ctx context.Context) error {
			if err := csvClients(ctx); err != nil {
				return err
			}
			return goldreport.Export(ctx, dest, src, folder)
		},
		check: func(ctx context.Context, previous map[string]string) ([]goldverify.CSVResult, map[string]string, error) {
			if err := csvClients(ctx); err != nil {
				return nil, nil, err
			}
			return goldverify.CheckDrive(ctx, src, dest, folder, *empty, previous)
		},
	}
	return finish(verify(ctx, &r, verificationOptions{mutate: *mutate, allowEmpty: *empty}, ops))
}
