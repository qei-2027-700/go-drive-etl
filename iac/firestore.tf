# Firestore API の有効化。
# BigQuery の API は手作業で有効化されているが、以降は Terraform 側で管理する。
#
# disable_on_destroy = false: terraform destroy でこの定義を消しても API は有効なまま残す。
# API を無効化すると、同じプロジェクト内で Terraform 管理外の資源まで巻き込んで壊れるため。
resource "google_project_service" "firestore" {
  project            = var.project_id
  service            = "firestore.googleapis.com"
  disable_on_destroy = false
}

# 状態管理（ファイルの同期状況・重複排除）に使う Firestore データベース。
# ドキュメント ID には drive_file_id を使うため、コレクションは files ひとつだけ。
resource "google_firestore_database" "default" {
  project     = var.project_id
  name        = "(default)"
  location_id = var.location
  type        = "FIRESTORE_NATIVE"

  # 誤削除防止。意図的に削除するときは、先にこの値を DELETE_PROTECTION_DISABLED にして
  # apply してから destroy する必要がある。
  delete_protection_state = "DELETE_PROTECTION_ENABLED"

  # Terraform の管理から外すときにデータベース自体も削除する。
  # 状態管理のデータは Drive から再構築できるため、残す価値が無い。
  deletion_policy = "DELETE"

  depends_on = [google_project_service.firestore]
}
