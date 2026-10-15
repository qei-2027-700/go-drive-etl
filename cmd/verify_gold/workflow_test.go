package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/qei-2027-700/go-drive-etl/internal/goldverify"
)

func TestVerificationFlow(t *testing.T) {
	tests := []struct {
		name        string
		mutate      bool
		failure     string
		calls       []string
		failedStage int
	}{
		{name: "read only", calls: []string{"compare", "check1"}},
		{name: "refresh and export twice", mutate: true, calls: []string{"refresh", "compare", "export1", "check1", "export2", "check2"}},
		{name: "refresh failure", mutate: true, failure: "refresh", calls: []string{"refresh"}, failedStage: refreshStage},
		{name: "comparison authentication failure", mutate: true, failure: "compare", calls: []string{"refresh", "compare"}, failedStage: comparisonStage},
		{name: "Gold mismatch", mutate: true, failure: "mismatch", calls: []string{"refresh", "compare"}, failedStage: comparisonStage},
		{name: "first export failure", mutate: true, failure: "export1", calls: []string{"refresh", "compare", "export1"}, failedStage: firstCSVStage},
		{name: "first download failure", mutate: true, failure: "check1", calls: []string{"refresh", "compare", "export1", "check1"}, failedStage: firstCSVStage},
		{name: "second export failure", mutate: true, failure: "export2", calls: []string{"refresh", "compare", "export1", "check1", "export2"}, failedStage: secondCSVStage},
		{name: "second check failure", mutate: true, failure: "check2", calls: []string{"refresh", "compare", "export1", "check1", "export2", "check2"}, failedStage: secondCSVStage},
		{name: "read only failure", failure: "check1", calls: []string{"compare", "check1"}, failedStage: firstCSVStage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			sentinel := errors.New("invalid_grant: synthetic failure")
			call := func(name string) error {
				calls = append(calls, name)
				if tc.failure == name {
					return sentinel
				}
				return nil
			}
			ids := map[string]string{"mart_ingestion_daily": "synthetic-a", "mart_file_latest": "synthetic-b"}
			exports, checks := 0, 0
			ops := verificationOps{
				refresh: func(context.Context) error { return call("refresh") },
				compare: func(context.Context) (map[string]int64, error) {
					err := call("compare")
					counts := map[string]int64{"silver_chunks": 4, "expected_daily": 1, "actual_daily": 1, "expected_latest": 2, "actual_latest": 2}
					if tc.failure == "mismatch" {
						counts["daily_missing"] = 1
					}
					return counts, err
				},
				export: func(context.Context) error {
					exports++
					if exports == 1 {
						return call("export1")
					}
					return call("export2")
				},
				check: func(_ context.Context, previous map[string]string) ([]goldverify.CSVResult, map[string]string, error) {
					checks++
					if checks == 1 && previous != nil {
						t.Fatal("first check received previous IDs")
					}
					if checks == 2 && !reflect.DeepEqual(previous, ids) {
						t.Fatal("second check lost first-round IDs")
					}
					name := "check1"
					if checks == 2 {
						name = "check2"
					}
					return []goldverify.CSVResult{{Table: "mart_ingestion_daily", Rows: 1, Columns: 6, Match: tc.failure != name}}, ids, call(name)
				},
			}
			r := newReport(tc.mutate)
			err := verify(context.Background(), &r, verificationOptions{mutate: tc.mutate}, ops)
			if !reflect.DeepEqual(calls, tc.calls) {
				t.Fatalf("calls %v; want %v", calls, tc.calls)
			}
			if (err != nil) != (tc.failure != "") {
				t.Fatalf("err=%v", err)
			}
			if tc.failure == "" {
				if r.Status != "partial" {
					t.Fatalf("status=%s", r.Status)
				}
			} else {
				if r.Status != "failed" || r.Stages[tc.failedStage].Status != "failed" {
					t.Fatal("failure not reflected in report")
				}
				if tc.failure != "mismatch" && (!errors.Is(err, sentinel) || r.Stages[tc.failedStage].ErrorCode != "invalid_grant") {
					t.Fatal("error details lost")
				}
				for _, st := range r.Stages[tc.failedStage+1 : dashboardStage] {
					if st.Status != "not_run" && st.Status != "not_requested" {
						t.Fatalf("later stage changed: %+v", st)
					}
				}
			}
			seen := map[string]bool{}
			for _, st := range r.Stages {
				if seen[st.Name] {
					t.Fatalf("duplicate stage %s", st.Name)
				}
				seen[st.Name] = true
			}
			if len(r.Stages) != 6 {
				t.Fatalf("unexpected stage count: %d", len(r.Stages))
			}
			if r.Stages[dashboardStage].Status != "unverified" {
				t.Fatal("API check must not verify dashboard")
			}
			if !tc.mutate && (r.Stages[refreshStage].Status != "not_requested" || r.Stages[secondCSVStage].Status != "not_requested") {
				t.Fatal("unrequested writes mislabeled")
			}
			if tc.failure == "check1" && len(r.Stages[firstCSVStage].CSV) != 1 {
				t.Fatal("partial CSV metrics lost on failure")
			}
			if tc.failure == "mismatch" && r.Stages[comparisonStage].Counts["daily_missing"] != 1 {
				t.Fatal("SQL differences lost")
			}
		})
	}
}
