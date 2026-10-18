# Normal views require access to every referenced table/view. Table-level grants
# keep BI's dataset access unchanged and do not authorize views for other users.
resource "google_service_account" "retro_app" {
  account_id   = "reflection-documents-app"
  display_name = "Retro documents application reader"
  description  = "Read-only document and upstream chunk access for the external application."
}

resource "google_project_iam_member" "retro_app_job_user" {
  project = var.project_id
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:${google_service_account.retro_app.email}"
}

locals {
  retro_app_tables = {
    chunks          = google_bigquery_table.chunks
    chunks_current  = google_bigquery_table.chunks_current
    silver_chunks   = google_bigquery_table.silver_chunks
    retro_documents = google_bigquery_table.retro_documents
  }
}

resource "google_bigquery_table_iam_member" "retro_app_reader" {
  for_each = local.retro_app_tables

  project    = var.project_id
  dataset_id = each.value.dataset_id
  table_id   = each.value.table_id
  role       = "roles/bigquery.dataViewer"
  member     = "serviceAccount:${google_service_account.retro_app.email}"
}

# Explicit opt-in for local verification; never grant token creation project-wide.
resource "google_service_account_iam_member" "retro_app_verifier" {
  for_each = setunion(var.retro_app_impersonator_members, var.reflection_app_impersonator_members)

  service_account_id = google_service_account.retro_app.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}

output "retro_app_service_account_email" {
  value = google_service_account.retro_app.email
}

# Keyless impersonation and read-only effective-permission verification.
resource "google_project_service" "retro_app_access" {
  for_each = toset([
    "iamcredentials.googleapis.com",
    "policytroubleshooter.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# Compatibility output for existing callers.
output "reflection_app_service_account_email" {
  value = google_service_account.retro_app.email
}
