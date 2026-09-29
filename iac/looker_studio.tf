# The Data Portal (formerly Looker Studio) BigQuery connector runs queries with this dedicated
# identity when a data source uses service-account credentials.  It is kept
# separate from the ETL worker so that BI access cannot write pipeline data.
resource "google_service_account" "looker_studio" {
  account_id   = "looker-studio-bi"
  display_name = "Data Portal BI reader"
  description  = "Read-only access to Gold marts for Data Portal dashboards."
}

# A query needs permission to create a job in its billing project.  This role
# does not grant any access to BigQuery data by itself.
resource "google_project_iam_member" "looker_studio_job_user" {
  project = var.project_id
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:${google_service_account.looker_studio.email}"
}

# Keep data access at the Gold dataset boundary: Bronze and Silver content is
# deliberately not visible to the BI identity.
resource "google_bigquery_dataset_iam_member" "looker_studio_gold_viewer" {
  project    = var.project_id
  dataset_id = google_bigquery_dataset.gold.dataset_id
  role       = "roles/bigquery.dataViewer"
  member     = "serviceAccount:${google_service_account.looker_studio.email}"
}

# Data Portal's organization-scoped service agent must be able to mint a
# token for the dedicated identity. Its email is only known after it has been
# retrieved from Data Portal, so the binding is opt-in via a variable.
resource "google_service_account_iam_member" "looker_studio_service_agent" {
  for_each           = toset(var.looker_studio_service_agent_member == "" ? [] : [var.looker_studio_service_agent_member])
  service_account_id = google_service_account.looker_studio.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}

# Limit who can select this service account while creating or editing a Data
# Portal data source. Use group: principals for team administration.
resource "google_service_account_iam_member" "looker_studio_data_source_editor" {
  for_each           = var.looker_studio_data_source_editor_members
  service_account_id = google_service_account.looker_studio.name
  role               = "roles/iam.serviceAccountUser"
  member             = each.value
}

output "looker_studio_service_account_email" {
  description = "Select this account as the credentials for the Data Portal BigQuery data source."
  value       = google_service_account.looker_studio.email
}
