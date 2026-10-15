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
	"strings"
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
	r := report{Started: time.Now().UTC().Format(time.RFC3339), Status: "failed", DashboardURL: os.Getenv("GOLD_DASHBOARD_URL"), DashboardReason: "Browser rendering has not been checked. Open the report with a signed-in browser and compare the dashboard_files/dashboard_pending counts with all-date scorecards; check daily chart and latest-file table as described in docs/looker-studio.md."}
	if r.DashboardURL == "" {
		if data, err := os.ReadFile("docs/looker-studio.md"); err == nil {
			r.DashboardURL = regexp.MustCompile(`https://datastudio\.google\.com/[^\s]+`).FindString(string(data))
		}
	}
	finish := func(code int) int {
		// Include unattempted stages after early failures as well as completed ones.
		for _, name := range []string{"refresh", "Gold comparison", "CSV round 1", "CSV round 2", "dashboard"} {
			found := false
			for _, st := range r.Stages {
				if strings.HasPrefix(st.Name, name) {
					found = true
					break
				}
			}
			if !found {
				state := "not_run"
				if name == "dashboard" {
					state = "unverified"
				}
				if name == "CSV round 2" && !*mutate {
					state = "not_requested"
				}
				r.Stages = append(r.Stages, stage{Name: name, Status: state})
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
	fail := func(name string, err error) int {
		code := "api_or_configuration_error"
		for _, candidate := range []string{"invalid_grant", "invalid_client", "insufficientPermissions", "storageQuotaExceeded"} {
			if strings.Contains(err.Error(), candidate) {
				code = candidate
				break
			}
		}
		r.Stages = append(r.Stages, stage{Name: name, Status: "failed", ErrorCode: code})
		// API errors can contain document IDs, SQL, credentials or URLs. Only the
		// private error file receives details; stdout and summary contain stage names.
		_ = os.WriteFile(filepath.Join(dir, "error.txt"), []byte(err.Error()), 0600)
		return finish(1)
	}
	project := os.Getenv("BIGQUERY_PROJECT_ID")
	silver := value("BQ_SILVER_DATASET_ID", "etl_silver")
	gold := value("BIGQUERY_GOLD_DATASET_ID", "etl_gold")
	location := value("BQ_LOCATION", "asia-northeast1")
	folder := os.Getenv("DRIVE_EXPORT_FOLDER_ID")
	sql, err := goldverify.ComparisonSQL(project, silver, gold)
	if err != nil {
		return fail("configuration", err)
	}
	if folder == "" {
		return fail("configuration", fmt.Errorf("DRIVE_EXPORT_FOLDER_ID required"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if *mutate {
		cmd := exec.CommandContext(ctx, "bash", "scripts/refresh_gold_marts.sh")
		cmd.Env = append(os.Environ(), "BQ_PROJECT_ID="+project, "BQ_GOLD_DATASET_ID="+gold, "BQ_SILVER_DATASET_ID="+silver, "BQ_LOCATION="+location)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fail("refresh", fmt.Errorf("%w: %s", err, output))
		}
		r.Stages = append(r.Stages, stage{Name: "refresh", Status: "passed"})
	} else {
		r.Stages = append(r.Stages, stage{Name: "refresh", Status: "not_requested"})
	}
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return fail("BigQuery authentication", err)
	}
	defer client.Close()
	query := client.Query(sql)
	query.Location = location
	iter, err := query.Read(ctx)
	if err != nil {
		return fail("Gold comparison", err)
	}
	row := map[string]bigquery.Value{}
	if err = iter.Next(&row); err != nil {
		return fail("Gold comparison", err)
	}
	counts := map[string]int64{}
	for k, v := range row {
		n, ok := v.(int64)
		if !ok {
			return fail("Gold comparison", fmt.Errorf("unexpected count type"))
		}
		counts[k] = n
	}
	good := goldverify.GoldMatches(counts, *empty)
	state := "passed"
	if !good {
		state = "failed"
	}
	r.Stages = append(r.Stages, stage{Name: "Gold comparison", Status: state, Counts: counts})
	if !good {
		return finish(1)
	}
	src, err := bqclient.NewClient(ctx)
	if err != nil {
		return fail("CSV BigQuery authentication", err)
	}
	defer src.Close()
	dest, err := drive.NewClient(ctx)
	if err != nil {
		return fail("Drive authentication", err)
	}
	rounds := 1
	if *mutate {
		rounds = 2
	}
	var previous map[string]string
	for i := 1; i <= rounds; i++ {
		name := fmt.Sprintf("CSV round %d", i)
		if *mutate {
			if err = goldreport.Export(ctx, dest, src, folder); err != nil {
				return fail(name+" export", err)
			}
		}
		results, ids, err := goldverify.CheckDrive(ctx, src, dest, folder, *empty, previous)
		state := "passed"
		if err != nil {
			state = "failed"
		}
		r.Stages = append(r.Stages, stage{Name: name, Status: state, CSV: results})
		if err != nil {
			return fail(name, err)
		}
		previous = ids
	}
	if !*mutate {
		r.Stages = append(r.Stages, stage{Name: "repeat export / stable IDs", Status: "not_requested"})
	}
	r.Stages = append(r.Stages, stage{Name: "dashboard", Status: "unverified"})
	r.Status = "partial"
	return finish(2)
}
