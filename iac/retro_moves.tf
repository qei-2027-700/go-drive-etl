# Keep deployed resource identities while renaming Terraform addresses.

moved {
  from = google_bigquery_table.reflection_documents
  to   = google_bigquery_table.retro_documents_legacy
}

moved {
  from = google_service_account.reflection_app
  to   = google_service_account.retro_app
}

moved {
  from = google_project_iam_member.reflection_app_job_user
  to   = google_project_iam_member.retro_app_job_user
}

moved {
  from = google_bigquery_table_iam_member.reflection_app_reader
  to   = google_bigquery_table_iam_member.retro_app_reader
}

moved {
  from = google_service_account_iam_member.reflection_app_verifier
  to   = google_service_account_iam_member.retro_app_verifier
}

moved {
  from = google_project_service.reflection_app_access
  to   = google_project_service.retro_app_access
}
