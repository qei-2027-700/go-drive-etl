# Silver exposes one trusted, analysis-ready contract over the append-only
# Bronze chunk store. Consumers must read this view instead of raw chunks.
resource "google_project_service" "bigquery_data_transfer" {
  project            = var.project_id
  service            = "bigquerydatatransfer.googleapis.com"
  disable_on_destroy = false
}

resource "google_bigquery_dataset" "silver" {
  dataset_id = var.silver_dataset_id
  location   = var.location

  description = "Trusted Silver views derived from the raw ETL dataset."
}

resource "google_bigquery_table" "silver_chunks" {
  dataset_id          = google_bigquery_dataset.silver.dataset_id
  table_id            = "silver_chunks"
  deletion_protection = false

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

# Gold stores stable, physical Mart tables for BI and report delivery. It has
# no default table expiration: its retention must not inherit Bronze's 60-day
# Sandbox safeguard.
resource "google_bigquery_dataset" "gold" {
  dataset_id = var.gold_dataset_id
  location   = var.location

  description = "Analytics-ready Gold Mart tables for BI and CSV delivery."
}

resource "google_bigquery_data_transfer_config" "mart_ingestion_daily" {
  display_name           = "Refresh mart_ingestion_daily"
  data_source_id         = "scheduled_query"
  destination_dataset_id = google_bigquery_dataset.gold.dataset_id
  location               = var.location
  schedule               = var.mart_refresh_schedule

  params = {
    destination_table_name_template = "mart_ingestion_daily"
    write_disposition               = "WRITE_TRUNCATE"
    query                           = <<-SQL
      SELECT
        ingested_date,
        COUNT(DISTINCT file_id) AS file_count,
        COUNT(*) AS chunk_count,
        SUM(content_length) AS content_length,
        COUNTIF(is_embedded) AS embedded_chunk_count,
        COUNTIF(NOT is_embedded) AS pending_embedding_chunk_count
      FROM `${var.project_id}.${google_bigquery_dataset.silver.dataset_id}.silver_chunks`
      GROUP BY ingested_date
    SQL
  }

  depends_on = [
    google_bigquery_table.silver_chunks,
    google_project_service.bigquery_data_transfer,
  ]
}

resource "google_bigquery_data_transfer_config" "mart_file_latest" {
  display_name           = "Refresh mart_file_latest"
  data_source_id         = "scheduled_query"
  destination_dataset_id = google_bigquery_dataset.gold.dataset_id
  location               = var.location
  schedule               = var.mart_refresh_schedule

  params = {
    destination_table_name_template = "mart_file_latest"
    write_disposition               = "WRITE_TRUNCATE"
    query                           = <<-SQL
      SELECT
        file_id,
        ANY_VALUE(content_version) AS content_version,
        MAX(ingested_at) AS latest_ingested_at,
        COUNT(*) AS chunk_count,
        SUM(content_length) AS content_length,
        COUNTIF(is_embedded) AS embedded_chunk_count,
        COUNTIF(NOT is_embedded) AS pending_embedding_chunk_count
      FROM `${var.project_id}.${google_bigquery_dataset.silver.dataset_id}.silver_chunks`
      GROUP BY file_id
    SQL
  }

  depends_on = [
    google_bigquery_table.silver_chunks,
    google_project_service.bigquery_data_transfer,
  ]
}
