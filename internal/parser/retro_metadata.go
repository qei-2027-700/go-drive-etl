package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RetroLabel describes the title's month and sequence, not an exact period.
type RetroLabel struct {
	Month    string
	Sequence int32
}

// RetroPeriod contains inclusive Japanese calendar dates, not timestamps.
type RetroPeriod struct {
	StartDate string
	EndDate   string
}

// SprintPolicy applies only to titles created under the Monday-based convention.
// Its zero value disables calculation so legacy sequence numbers remain undated.
type SprintPolicy struct {
	fromMonth string
}

func NewSprintPolicy(fromMonth string) (SprintPolicy, error) {
	if fromMonth == "" {
		return SprintPolicy{}, nil
	}
	month, err := time.Parse("2006-01", fromMonth)
	if err != nil || month.Year() < 1 || month.Format("2006-01") != fromMonth {
		return SprintPolicy{}, fmt.Errorf("sprint start month must be YYYY-MM with year 0001 through 9999")
	}
	return SprintPolicy{fromMonth: fromMonth}, nil
}

// Period maps the Nth Monday of the title month to Monday through Sunday.
// An absent fifth Monday is invalid; it must not roll into the following month.
func (p SprintPolicy) Period(label *RetroLabel) *RetroPeriod {
	if p.fromMonth == "" || label == nil || label.Month < p.fromMonth || label.Sequence < 1 || label.Sequence > 5 {
		return nil
	}
	month, err := time.Parse("2006-01", label.Month)
	if err != nil || month.Year() < 1 || month.Format("2006-01") != label.Month {
		return nil
	}
	// These are date-only calculations. No ingestion timestamp or UTC/JST
	// conversion is involved in assigning the target calendar dates.
	offset := (int(time.Monday) - int(month.Weekday()) + 7) % 7
	start := month.AddDate(0, 0, offset+7*int(label.Sequence-1))
	end := start.AddDate(0, 0, 6)
	if start.Month() != month.Month() || end.Year() > 9999 {
		return nil
	}
	return &RetroPeriod{StartDate: start.Format(time.DateOnly), EndDate: end.Format(time.DateOnly)}
}

var retroHeading = regexp.MustCompile(`^# ([0-9]{4})(0[1-9]|1[0-2])-([1-9][0-9]*)$`)

// ExtractRetroLabel reads a YYYYMM-N H1 on the first nonblank line.
// Unrecognized titles remain undated; filenames and body dates are not fallbacks.
func ExtractRetroLabel(content []byte) *RetroLabel {
	text := strings.TrimPrefix(string(content), "\ufeff")
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Keep leading indentation: an indented code block is not a title.
		match := retroHeading.FindStringSubmatch(strings.TrimRight(line, " \t\r"))
		if match == nil || match[1] == "0000" {
			return nil
		}
		sequence, err := strconv.ParseInt(match[3], 10, 32)
		if err != nil {
			return nil
		}
		return &RetroLabel{Month: match[1] + "-" + match[2], Sequence: int32(sequence)}
	}
	return nil
}
