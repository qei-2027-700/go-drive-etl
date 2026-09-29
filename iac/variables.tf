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

variable "silver_dataset_id" {
  description = "Silver 層の BigQuery dataset ID"
  type        = string
  default     = "etl_silver"
}

variable "gold_dataset_id" {
  description = "Gold 層の BigQuery dataset ID"
  type        = string
  default     = "etl_gold"
}
