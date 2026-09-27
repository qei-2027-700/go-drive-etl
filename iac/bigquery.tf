resource "google_bigquery_dataset" "etl" {
  dataset_id = var.dataset_id
  location   = var.location
  # 課金未設定のため 60 日上限（課金有効化後は削除可）
  default_table_expiration_ms     = 5184000000
  default_partition_expiration_ms = 5184000000
}

resource "google_bigquery_table" "drive_files" {
  dataset_id          = google_bigquery_dataset.etl.dataset_id
  table_id            = "drive_files"
  deletion_protection = false

  schema = jsonencode([
    {
      name = "drive_file_id"
      type = "STRING"
      mode = "REQUIRED"
    },
    {
      name = "path"
      type = "STRING"
      mode = "NULLABLE"
    },
    {
      name = "checksum"
      type = "STRING"
      mode = "NULLABLE"
    },
    {
      name = "mime_type"
      type = "STRING"
      mode = "NULLABLE"
    },
    {
      name = "sync_status"
      type = "STRING"
      mode = "NULLABLE"
    },
    {
      name = "updated_at"
      type = "TIMESTAMP"
      mode = "NULLABLE"
    }
  ])
}

resource "google_bigquery_table" "chunks" {
  dataset_id          = google_bigquery_dataset.etl.dataset_id
  table_id            = "chunks"
  deletion_protection = false

  schema = jsonencode([
    {
      name = "file_id"
      type = "STRING"
      mode = "REQUIRED"
    },
    {
      name = "chunk_index"
      type = "INTEGER"
      mode = "REQUIRED"
    },
    {
      name = "content"
      type = "STRING"
      mode = "REQUIRED"
    },
    {
      name = "embedding_status"
      type = "STRING"
      mode = "REQUIRED"
    },
    {
      # NULLABLE allows this field to be added to an existing chunks table.
      name = "content_version"
      type = "STRING"
      mode = "NULLABLE"
    },
    {
      # NULLABLE allows this field to be added to an existing chunks table.
      name = "ingested_at"
      type = "TIMESTAMP"
      mode = "NULLABLE"
    },
    {
      # A tombstone represents a file whose current version contains no chunks.
      name = "is_deleted"
      type = "BOOLEAN"
      mode = "NULLABLE"
    }
  ])
}

resource "google_bigquery_table" "chunks_current" {
  dataset_id          = google_bigquery_dataset.etl.dataset_id
  table_id            = "chunks_current"
  deletion_protection = false

  view {
    query          = <<-SQL
      WITH version_candidates AS (
        SELECT file_id, content_version, MAX(ingested_at) AS version_ingested_at
        FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks`
        WHERE content_version IS NOT NULL
        GROUP BY file_id, content_version
      ),
      latest_versions AS (
        SELECT file_id, content_version
        FROM version_candidates
        QUALIFY ROW_NUMBER() OVER (
          PARTITION BY file_id ORDER BY version_ingested_at DESC, content_version DESC
        ) = 1
      ),
      deduplicated_chunks AS (
        SELECT file_id, chunk_index, content, embedding_status, content_version, ingested_at, is_deleted
        FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks`
        QUALIFY ROW_NUMBER() OVER (
          PARTITION BY file_id, content_version, chunk_index ORDER BY ingested_at DESC
        ) = 1
      )
      SELECT chunks.file_id, chunks.chunk_index, chunks.content, chunks.embedding_status,
        chunks.content_version, chunks.ingested_at
      FROM deduplicated_chunks AS chunks
      INNER JOIN latest_versions USING (file_id, content_version)
      WHERE COALESCE(chunks.is_deleted, FALSE) = FALSE
    SQL
    use_legacy_sql = false
  }
}
