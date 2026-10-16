package parser

import "testing"

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
		{"no title", "2026-01", "# Reflection\n# 202610-1", "", ""},
		{"bad month", "2026-01", "# 202613-1", "", ""},
		{"date exceeds BigQuery range", "9999-01", "# 999912-4", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := NewSprintPolicy(tc.from)
			if err != nil {
				t.Fatal(err)
			}
			got := policy.Period(ExtractReflectionLabel([]byte(tc.title)))
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
	for _, label := range []*ReflectionLabel{
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
