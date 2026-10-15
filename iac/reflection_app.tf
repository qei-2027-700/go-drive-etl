# Normal views require access to every referenced table/view. Table-level grants
# keep BI's dataset access unchanged and do not authorize views for other users.
resource "google_service_account" "reflection_app" {
  account_id   = "reflection-documents-app"
  display_name = "Reflection documents application reader"
  description  = "Read-only document and upstream chunk access for the external application."
}

resource "google_project_iam_member" "reflection_app_job_user" {
  project = var.project_id
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:${google_service_account.reflection_app.email}"
}

locals {
  reflection_app_tables = {
    chunks               = google_bigquery_table.chunks
    chunks_current       = google_bigquery_table.chunks_current
    silver_chunks        = google_bigquery_table.silver_chunks
    reflection_documents = google_bigquery_table.reflection_documents
  }
}

resource "google_bigquery_table_iam_member" "reflection_app_reader" {
  for_each = local.reflection_app_tables

  project    = var.project_id
  dataset_id = each.value.dataset_id
  table_id   = each.value.table_id
  role       = "roles/bigquery.dataViewer"
  member     = "serviceAccount:${google_service_account.reflection_app.email}"
}

# Explicit opt-in for local verification; never grant token creation project-wide.
resource "google_service_account_iam_member" "reflection_app_verifier" {
  for_each = var.reflection_app_impersonator_members

  service_account_id = google_service_account.reflection_app.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}

output "reflection_app_service_account_email" {
  value = google_service_account.reflection_app.email
}

# Keyless impersonation and read-only effective-permission verification.
resource "google_project_service" "reflection_app_access" {
  for_each = toset([
    "iamcredentials.googleapis.com",
    "policytroubleshooter.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}
