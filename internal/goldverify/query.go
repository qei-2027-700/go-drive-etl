package goldverify

import (
	"fmt"
	"regexp"
	"strings"
)

var projectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,61}[a-z0-9]$`)
var datasetPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Validate(project, silver, gold string) error {
	if !projectPattern.MatchString(project) || !datasetPattern.MatchString(silver) || !datasetPattern.MatchString(gold) {
		return fmt.Errorf("invalid project/dataset configuration")
	}
	return nil
}

// ComparisonSQL independently computes expectations from Silver. Difference
// counts cover every column in both directions; row counts also catch duplicates.
func ComparisonSQL(project, silver, gold string) (string, error) {
	if err := Validate(project, silver, gold); err != nil {
		return "", err
	}
	sql := fmt.Sprintf(`WITH
 daily AS (
 SELECT ingested_date, COUNT(DISTINCT file_id) file_count, COUNT(*) chunk_count,
 SUM(content_length) content_length, COUNTIF(is_embedded) embedded_chunk_count,
 COUNTIF(NOT is_embedded) pending_embedding_chunk_count
 FROM %[1]s.%[2]s.silver_chunks GROUP BY ingested_date),
 latest AS (
 SELECT file_id, ANY_VALUE(content_version) content_version, MAX(ingested_at) latest_ingested_at,
 COUNT(*) chunk_count, SUM(content_length) content_length, COUNTIF(is_embedded) embedded_chunk_count,
 COUNTIF(NOT is_embedded) pending_embedding_chunk_count
 FROM %[1]s.%[2]s.silver_chunks GROUP BY file_id),
 d_missing AS (SELECT * FROM daily EXCEPT DISTINCT SELECT * FROM %[1]s.%[3]s.mart_ingestion_daily),
 d_extra AS (SELECT * FROM %[1]s.%[3]s.mart_ingestion_daily EXCEPT DISTINCT SELECT * FROM daily),
 f_missing AS (SELECT * FROM latest EXCEPT DISTINCT SELECT * FROM %[1]s.%[3]s.mart_file_latest),
 f_extra AS (SELECT * FROM %[1]s.%[3]s.mart_file_latest EXCEPT DISTINCT SELECT * FROM latest)
 SELECT
 (SELECT COUNT(*) FROM %[1]s.%[2]s.silver_chunks) silver_chunks,
 (SELECT COUNT(*) FROM daily) expected_daily,
 (SELECT COUNT(*) FROM latest) expected_latest,
 (SELECT COUNT(*) FROM %[1]s.%[3]s.mart_ingestion_daily) actual_daily,
 (SELECT COUNT(*) FROM %[1]s.%[3]s.mart_file_latest) actual_latest,
 (SELECT COUNT(*) FROM d_missing) daily_missing,
 (SELECT COUNT(*) FROM d_extra) daily_extra,
 (SELECT COUNT(*) FROM f_missing) latest_missing,
 (SELECT COUNT(*) FROM f_extra) latest_extra,
 (SELECT COALESCE(SUM(file_count),0) FROM daily) dashboard_files,
 (SELECT COALESCE(SUM(pending_embedding_chunk_count),0) FROM daily) dashboard_pending`, project, silver, gold)
	for _, path := range []string{project + "." + silver + ".silver_chunks", project + "." + gold + ".mart_ingestion_daily", project + "." + gold + ".mart_file_latest"} {
		sql = strings.ReplaceAll(sql, path, "`"+path+"`")
	}
	return sql, nil
}
