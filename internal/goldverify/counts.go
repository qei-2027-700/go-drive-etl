package goldverify

// GoldMatches distinguishes unexpected empty inputs, stale totals, full-row
// differences and duplicate Gold rows without exposing any source records.
func GoldMatches(c map[string]int64, allowEmpty bool) bool {
	for _, key := range []string{"daily_missing", "daily_extra", "latest_missing", "latest_extra"} {
		if c[key] != 0 {
			return false
		}
	}
	return c["expected_daily"] == c["actual_daily"] && c["expected_latest"] == c["actual_latest"] && (allowEmpty || c["silver_chunks"] > 0)
}
