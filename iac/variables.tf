variable "project_id" {
  description = "GCP プロジェクト ID"
  type        = string
}

variable "dataset_id" {
  description = "BigQuery データセット ID"
  type        = string
  default     = "etl_raw"
}

variable "location" {
  description = "リージョン"
  type        = string
  default     = "asia-northeast1"
}
