resource "google_bigquery_dataset" "silver" {
  dataset_id = var.silver_dataset_id
  location   = var.location

  description = "Trusted Silver views derived from the raw ETL dataset."

  # Required while the project uses BigQuery Sandbox without billing.
  default_table_expiration_ms     = 5184000000
  default_partition_expiration_ms = 5184000000
}

resource "google_bigquery_table" "silver_chunks" {
  dataset_id          = google_bigquery_dataset.silver.dataset_id
  table_id            = "silver_chunks"
  deletion_protection = true

  view {
    query = <<-SQL
      SELECT
        file_id,
        chunk_index,
        content,
        content_version,
        ingested_at,
        DATE(ingested_at, "Asia/Tokyo") AS ingested_date,
        embedding_status,
        embedding_status = "completed" AS is_embedded,
        LENGTH(content) AS content_length
      FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks_current`
    SQL

    use_legacy_sql = false
  }
}

# BigQuery Sandbox requires table expiration to be at most 60 days. Keep this
# explicit until billing is enabled; Gold marts are rebuilt before BI use.
resource "google_bigquery_dataset" "gold" {
  dataset_id = var.gold_dataset_id
  location   = var.location

  description = "Analytics-ready Gold Mart tables for BI and CSV delivery."

  default_table_expiration_ms     = 5184000000
  default_partition_expiration_ms = 5184000000
}
