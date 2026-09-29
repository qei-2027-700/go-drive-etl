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

variable "looker_studio_service_agent_member" {
  description = "Data Portal (formerly Looker Studio) service agent principal (serviceAccount:service-org-<ORG_ID>@gcp-sa-datastudio.iam.gserviceaccount.com). Empty skips its impersonation binding."
  type        = string
  default     = ""
}

variable "looker_studio_data_source_editor_members" {
  description = "Principals allowed to select the BI service account in Data Portal (for example, group:bi-admins@example.com)."
  type        = set(string)
  default     = []
}
