resource "google_bigquery_dataset" "silver" {
  dataset_id = var.silver_dataset_id
  location   = var.location

  description = "Trusted Silver views derived from the raw ETL dataset."

  # Required while the project uses BigQuery Sandbox without billing.
  default_table_expiration_ms     = 5184000000
  default_partition_expiration_ms = 5184000000
}

resource "google_bigquery_table" "silver_chunks" {
  depends_on = [google_bigquery_table.chunks_current]

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
        document_name,
        drive_url,
        retro_start_date,
        retro_end_date,
        retro_month,
        retro_sequence,
        DATE(ingested_at, "Asia/Tokyo") AS ingested_date,
        embedding_status,
        embedding_status = "completed" AS is_embedded,
        LENGTH(content) AS content_length
      FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks_current`
    SQL

    use_legacy_sql = false
  }
}

resource "google_bigquery_table" "retro_documents" {
  depends_on = [google_bigquery_table.silver_chunks]

  dataset_id          = google_bigquery_dataset.gold.dataset_id
  table_id            = "retro_documents"
  deletion_protection = true

  view {
    query = <<-SQL
      WITH documents AS (
        SELECT
          file_id,
          STRING_AGG(content, '\n\n' ORDER BY chunk_index) AS content,
          MAX(ingested_at) AS ingested_at,
          ARRAY_AGG(
            STRUCT(
              document_name,
              drive_url,
              content_version,
              retro_month,
              retro_sequence,
              retro_start_date,
              retro_end_date
            )
            ORDER BY ingested_at DESC, chunk_index ASC
            LIMIT 1
          )[OFFSET(0)] AS metadata
        FROM `${var.project_id}.${google_bigquery_dataset.silver.dataset_id}.silver_chunks`
        GROUP BY file_id
      )
      SELECT
        file_id,
        metadata.document_name,
        metadata.drive_url,
        content,
        metadata.content_version,
        ingested_at,
        metadata.retro_month,
        metadata.retro_sequence,
        metadata.retro_start_date,
        metadata.retro_end_date
      FROM documents
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
