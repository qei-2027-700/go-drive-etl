-- Rebuild the Gold Marts from the Silver view.
-- Parameters supplied by scripts/refresh_gold_marts.sh:
--   @project_id, @silver_dataset_id, @gold_dataset_id
--
-- CREATE OR REPLACE TABLE replaces each table atomically after its query
-- succeeds. This is equivalent to the prior Scheduled Query WRITE_TRUNCATE
-- behavior, without requiring BigQuery Data Transfer Service or Billing.

EXECUTE IMMEDIATE FORMAT("""
  CREATE OR REPLACE TABLE `%s.%s.mart_ingestion_daily` AS
  SELECT
    ingested_date,
    COUNT(DISTINCT file_id) AS file_count,
    COUNT(*) AS chunk_count,
    SUM(content_length) AS content_length,
    COUNTIF(is_embedded) AS embedded_chunk_count,
    COUNTIF(NOT is_embedded) AS pending_embedding_chunk_count
  FROM `%s.%s.silver_chunks`
  GROUP BY ingested_date
""", @project_id, @gold_dataset_id, @project_id, @silver_dataset_id);

EXECUTE IMMEDIATE FORMAT("""
  CREATE OR REPLACE TABLE `%s.%s.mart_file_latest` AS
  SELECT
    file_id,
    ANY_VALUE(content_version) AS content_version,
    MAX(ingested_at) AS latest_ingested_at,
    COUNT(*) AS chunk_count,
    SUM(content_length) AS content_length,
    COUNTIF(is_embedded) AS embedded_chunk_count,
    COUNTIF(NOT is_embedded) AS pending_embedding_chunk_count
  FROM `%s.%s.silver_chunks`
  GROUP BY file_id
""", @project_id, @gold_dataset_id, @project_id, @silver_dataset_id);
