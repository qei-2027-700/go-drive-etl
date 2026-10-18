package parser

import "testing"

func TestExtractRetroLabel(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		month    string
		sequence int32
	}{
		{"monthly label", "# 202610-1\nSummary.", "2026-10", 1},
		{"sequence is not a week number", "# 202609-12\nSummary.", "2026-09", 12},
		{"CRLF and leading blank line", "\r\n# 202601-2\r\nSummary.", "2026-01", 2},
		{"UTF-8 BOM", "\ufeff# 202612-3\nSummary.", "2026-12", 3},
		{"whitespace-only leading lines", " \t\n# 202610-1\nSummary.", "2026-10", 1},
		{"indented code is not a heading", "    # 202610-1\nSummary.", "", 0},
		{"tab-indented code is not a heading", "\t# 202610-1\nSummary.", "", 0},
		{"no heading", "202610-1\nSummary.", "", 0},
		{"unrelated title", "# Retro\n## 202610-1", "", 0},
		{"body date is not a title", "# Retro\n# 202610-1", "", 0},
		{"subheading", "## 202610-1", "", 0},
		{"invalid month", "# 202613-1", "", 0},
		{"zero month", "# 202600-1", "", 0},
		{"zero year", "# 000010-1", "", 0},
		{"zero sequence", "# 202610-0", "", 0},
		{"overflow sequence", "# 202610-2147483648", "", 0},
		{"incomplete label", "# 202610-", "", 0},
		{"trailing title text", "# 202610-1 other title", "", 0},
		{"empty", " \n\t", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractRetroLabel([]byte(tc.content))
			if tc.month == "" {
				if got != nil {
					t.Fatalf("label = %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Month != tc.month || got.Sequence != tc.sequence {
				t.Fatalf("label = %+v, want %s/%d", got, tc.month, tc.sequence)
			}
		})
	}
}

func TestSprintPolicyPeriods(t *testing.T) {
	cases := []struct {
		name, from, title, start, end string
	}{
		{"disabled", "", "# 202610-1", "", ""},
		{"legacy month", "2026-11", "# 202610-1", "", ""},
		{"agreed November cutover", "2026-11", "# 202611-1", "2026-11-02", "2026-11-08"},
		{"cutover month", "2026-10", "# 202610-1", "2026-10-05", "2026-10-11"},
		{"following month", "2026-10", "# 202611-1", "2026-11-02", "2026-11-08"},
		{"first day Monday", "2026-01", "# 202606-1", "2026-06-01", "2026-06-07"},
		{"first day Saturday", "2026-01", "# 202608-1", "2026-08-03", "2026-08-09"},
		{"Tuesday first belongs to prior month", "2026-01", "# 202608-5", "2026-08-31", "2026-09-06"},
		{"month crossing", "2026-01", "# 202609-4", "2026-09-28", "2026-10-04"},
		{"year crossing", "2026-01", "# 202612-4", "2026-12-28", "2027-01-03"},
		{"leap day", "2024-01", "# 202402-4", "2024-02-26", "2024-03-03"},
		{"missing fifth Monday", "2026-01", "# 202610-5", "", ""},
		{"sequence is a legacy counter", "2026-01", "# 202610-12", "", ""},
		{"no title", "2026-01", "# Retro\n# 202610-1", "", ""},
		{"bad month", "2026-01", "# 202613-1", "", ""},
		{"date exceeds BigQuery range", "9999-01", "# 999912-4", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := NewSprintPolicy(tc.from)
			if err != nil {
				t.Fatal(err)
			}
			got := policy.Period(ExtractRetroLabel([]byte(tc.title)))
			if tc.start == "" {
				if got != nil {
					t.Fatalf("period = %+v, want nil", got)
				}
				return
			}
			if got == nil || got.StartDate != tc.start || got.EndDate != tc.end {
				t.Fatalf("period = %+v, want %s through %s", got, tc.start, tc.end)
			}
		})
	}
}

func TestSprintPolicyRejectsInvalidConfiguration(t *testing.T) {
	for _, value := range []string{"2026-1", "2026-00", "2026-13", "0000-01", "2026-10-01", " 2026-10", "2026-10 "} {
		if _, err := NewSprintPolicy(value); err == nil {
			t.Errorf("NewSprintPolicy(%q) should fail", value)
		}
	}
}

func TestSprintPolicyRejectsInvalidLabels(t *testing.T) {
	policy, err := NewSprintPolicy("2026-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []*RetroLabel{
		nil,
		{Month: "2026-13", Sequence: 1},
		{Month: "2026-1", Sequence: 1},
		{Month: "2026-10", Sequence: 0},
		{Month: "2026-10", Sequence: 2147483647},
	} {
		if got := policy.Period(label); got != nil {
			t.Errorf("Period(%+v) = %+v, want nil", label, got)
		}
	}
}
