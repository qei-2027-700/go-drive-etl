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
