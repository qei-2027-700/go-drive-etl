package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/qei-2027-700/go-drive-etl/internal/goldverify"
)

const (
	configurationStage = iota
	refreshStage
	comparisonStage
	firstCSVStage
	secondCSVStage
	dashboardStage
)

type verificationOptions struct{ mutate, allowEmpty bool }
type verificationOps struct {
	refresh func(context.Context) error
	compare func(context.Context) (map[string]int64, error)
	export  func(context.Context) error
	check   func(context.Context, map[string]string) ([]goldverify.CSVResult, map[string]string, error)
}

func newReport(mutate bool) report {
	r := report{Status: "failed", Stages: []stage{
		{Name: "configuration", Status: "not_run"},
		{Name: "refresh", Status: "not_run"},
		{Name: "Gold comparison", Status: "not_run"},
		{Name: "CSV round 1", Status: "not_run"},
		{Name: "CSV round 2", Status: "not_run"},
		{Name: "dashboard", Status: "unverified"},
	}}
	if !mutate {
		r.Stages[refreshStage].Status = "not_requested"
		r.Stages[secondCSVStage].Status = "not_requested"
	}
	return r
}

func (r *report) fail(index int, err error) error {
	st := &r.Stages[index]
	st.Status = "failed"
	st.ErrorCode = "api_or_configuration_error"
	for _, code := range []string{"invalid_grant", "invalid_client", "insufficientPermissions", "storageQuotaExceeded"} {
		if strings.Contains(err.Error(), code) {
			st.ErrorCode = code
			break
		}
	}
	return err
}

// verify updates one preallocated result per stage and stops at the first failure.
// All cloud effects are supplied by ops so tests exercise the production ordering.
func verify(ctx context.Context, r *report, options verificationOptions, ops verificationOps) error {
	if options.mutate {
		if err := ops.refresh(ctx); err != nil {
			return r.fail(refreshStage, err)
		}
		r.Stages[refreshStage].Status = "passed"
	}
	counts, err := ops.compare(ctx)
	r.Stages[comparisonStage].Counts = counts
	if err != nil {
		return r.fail(comparisonStage, err)
	}
	if !goldverify.GoldMatches(counts, options.allowEmpty) {
		return r.fail(comparisonStage, fmt.Errorf("Gold differs from Silver or input is unexpectedly empty"))
	}
	r.Stages[comparisonStage].Status = "passed"
	rounds := 1
	if options.mutate {
		rounds = 2
	}
	var previous map[string]string
	for i := 0; i < rounds; i++ {
		index := firstCSVStage + i
		if options.mutate {
			if err := ops.export(ctx); err != nil {
				return r.fail(index, err)
			}
		}
		results, ids, err := ops.check(ctx, previous)
		r.Stages[index].CSV = results
		if err != nil {
			return r.fail(index, err)
		}
		r.Stages[index].Status = "passed"
		previous = ids
	}
	r.Status = "partial"
	return nil
}
