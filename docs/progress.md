# 進捗管理

> GCP プロジェクト: **`go-drive-etl`**
> 最終更新: 2026-05-30（OAuth クライアント ID 確認済み）

---

## フェーズ別ステータス

### Phase 1: プロジェクト基盤

| # | タスク | 状態 |
|:---:|:---|:---:|
| 1-1 | Go モジュール初期化 (`go mod init`) | ✅ |
| 1-2 | ディレクトリ構成作成 | 🚧 一部未作成 |
| 1-3 | `docker-compose.yml` (PostgreSQL) | ✅ |

> **未作成ディレクトリ**: `proto/`, `internal/bq/`, `internal/etl/`, `internal/parser/`, `internal/pb/`, `infra/`（`iac/` として存在するが空）

---

### Phase 2: スキーマ定義 (Protocol Buffers)

| # | タスク | 状態 |
|:---:|:---|:---:|
| 2-1 | `proto/record.proto` 作成 | ❌ |
| 2-2 | `protoc` コンパイル環境構築 | ❌ |
| 2-3 | `internal/pb/` にコード生成 | ❌ |

---

### Phase 3: DB マイグレーション

| # | タスク | 状態 |
|:---:|:---|:---:|
| 3-1 | `migrations/001_init.sql` 作成 | ✅ |
| 3-2 | Postgres 起動 & マイグレーション適用 | 🚧 要確認 |

---

### Phase 4: Repository (PostgreSQL)

| # | タスク | 状態 |
|:---:|:---|:---:|
| 4-1 | `internal/domain/file.go` 型定義 | ✅ |
| 4-2 | `FileRepository.Upsert` | ✅ |
| 4-3 | `FileRepository.ListPending` | 🐛 バグあり |
| 4-4 | `SyncStatus` 定数の有効化 | 🚧 コメントアウト中 |
| 4-5 | `UpdateStatus` メソッド実装 | ❌ |

> **🐛 バグ**: `ListPending` の `rows.Scan` が引数を重複指定 → ランタイムエラー

---

### Phase 5: Google Drive クライアント

| # | タスク | 状態 |
|:---:|:---|:---:|
| 5-1 | GCP: Drive API 有効化 | ✅ |
| 5-2 | GCP: OAuth 同意画面 構成 | ✅ |
| 5-3 | GCP: OAuth 2.0 クライアント ID 作成 (`go-drive-etl-key`) | ✅ |
| 5-4 | `cmd/auth/main.go` でリフレッシュトークン取得 | ❓ 要確認 |
| 5-5 | `.env` に認証情報設定 (`CLIENT_ID` / `SECRET` / `REFRESH_TOKEN`) | ❓ 要確認 |
| 5-6 | `internal/drive/client.go` 本実装 | ❌ |

---

### Phase 6: Worker Pool (Go Concurrency)

| # | タスク | 状態 |
|:---:|:---|:---:|
| 6-1 | `internal/etl/worker_pool.go` 作成 | ❌ |
| 6-2 | `ctx.Done()` 監視 / Graceful Shutdown | ❌ |

---

### Phase 7: BigQuery クライアント

| # | タスク | 状態 |
|:---:|:---|:---:|
| 7-1 | GCP: BigQuery API 有効化 | ✅（多分） |
| 7-2 | GCP: BigQuery データセット作成 | ❓ 要確認 |
| 7-3 | `go get cloud.google.com/go/bigquery` | ❌ (`go.mod` 未追加) |
| 7-4 | `internal/bq/client.go` 作成 | ❌ |

---

### Phase 8: ETL パイプライン統合

| # | タスク | 状態 |
|:---:|:---|:---:|
| 8-1 | `cmd/worker/main.go` 作成 | ❌ |
| 8-2 | Drive → Postgres → Proto → BQ の End-to-End 疎通 | ❌ |
| 8-3 | Graceful Shutdown テスト | ❌ |

---

### IaC (Terraform)

| # | タスク | 状態 |
|:---:|:---|:---:|
| T-1 | `iac/main.tf` 作成 (BigQuery データセット / テーブル定義) | ❌ |

---

## 凡例

| 記号 | 意味 |
|:---:|:---|
| ✅ | 完了 |
| 🚧 | 着手済み・一部未完了 |
| ❌ | 未着手 |
| ❓ | 要確認 |
| 🐛 | バグあり |
