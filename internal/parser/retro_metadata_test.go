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
