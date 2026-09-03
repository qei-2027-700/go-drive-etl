# 進捗管理

> GCP プロジェクト: **`go-drive-etl`**
> 最終更新: **2026-09-02（実装状況と全面的に再同期）**
>
> ⚠️ **このファイルは 2026-05-30 以降更新されておらず、実装が進んだのに「未着手」のままの項目が多数あった。**
> ソースコードを確認して実態に合わせた。**次回以降、PRマージ時にこのファイルも更新すること。**

---

## 🔴 残っている本丸

**パイプラインの各部品は揃っているが、それを起動するエントリポイントが無い。**

| # | 残タスク | なぜ重要か |
|:---:|:---|:---|
| **8-1** | `cmd/worker/main.go`（Issue #6・未着手） | **これが無いと「動くパイプライン」と言えない。** 部品はあるが通しで実行できない |
| **8-2** | Drive → Postgres → Proto → BQ の End-to-End 疎通 | 同上。**ポートフォリオとしての価値はここで決まる** |
| 2-2 | `protoc` 生成手順が `Makefile` に無い | 生成物（`internal/pb/`）はあるが**再生成が再現できない** |
| 1-2 | `internal/parser/` 未作成 | Issue #25（ファイルパーサ・チャンク化）が未着手のため |

---

## フェーズ別ステータス

### Phase 1: プロジェクト基盤

| # | タスク | 状態 |
|:---:|:---|:---:|
| 1-1 | Go モジュール初期化 (`go mod init`) | ✅ |
| 1-2 | ディレクトリ構成作成 | 🚧 `internal/parser/` のみ未作成 |
| 1-3 | `docker-compose.yml` (PostgreSQL) | ✅ |

> **作成済み**: `proto/` `internal/bq/` `internal/etl/` `internal/pb/` `internal/domain/` `internal/repository/` `internal/drive/` `iac/`
> **未作成**: `internal/parser/`（Issue #25）

---

### Phase 2: スキーマ定義 (Protocol Buffers)

| # | タスク | 状態 |
|:---:|:---|:---:|
| 2-1 | `proto/record.proto` 作成 | ✅ `FileRecord` / `ChunkRecord` / `SyncStatus` を定義 |
| 2-2 | `protoc` コンパイル環境構築 | 🚧 **生成物はあるが `Makefile` に生成ターゲットが無い**（`mock` のみ） |
| 2-3 | `internal/pb/` にコード生成 | ✅ `record.pb.go` |

> ℹ️ **`service` 定義は無い＝gRPCは使っていない。** スキーマ定義としてのみ protobuf を採用している。
> この区別は面接で必ず説明できるようにする（`carrier-advice` の `interview/qa-backend.md`）。

---

### Phase 3: DB マイグレーション

| # | タスク | 状態 |
|:---:|:---|:---:|
| 3-1 | `migrations/001_init.sql` 作成 | ✅ |
| 3-2 | Postgres 起動 & マイグレーション適用 | ❓ 要確認（ローカル実行の確認が必要） |

---

### Phase 4: Repository (PostgreSQL)

| # | タスク | 状態 |
|:---:|:---|:---:|
| 4-1 | `internal/domain/file.go` 型定義 | ✅ |
| 4-2 | `FileRepository.Upsert` | ✅ |
| 4-3 | `FileRepository.ListPending` | ✅ **バグ解消済み**（`rows.Scan` の重複指定は解消） |
| 4-4 | `SyncStatus` 定数の有効化 | ✅ `domain` パッケージに定数化済み |
| 4-5 | `UpdateStatus` メソッド実装 | ✅ |

---

### Phase 5: Google Drive クライアント

| # | タスク | 状態 |
|:---:|:---|:---:|
| 5-1 | GCP: Drive API 有効化 | ✅ |
| 5-2 | GCP: OAuth 同意画面 構成 | ✅ |
| 5-3 | GCP: OAuth 2.0 クライアント ID 作成 (`go-drive-etl-key`) | ✅ |
| 5-4 | `cmd/auth/main.go` でリフレッシュトークン取得 | ✅ 実装済み |
| 5-5 | `.env` に認証情報設定 | ✅ ローカルに `.env` あり（`.env.example` も整備済み） |
| 5-6 | `internal/drive/client.go` 本実装 | ✅ `NewClient` / `ListFiles` / `DownloadFile` |

---

### Phase 6: Worker Pool (Go Concurrency)

| # | タスク | 状態 |
|:---:|:---|:---:|
| 6-1 | `internal/etl/worker_pool.go` 作成 | ✅ `Run` 実装済み |
| 6-2 | `ctx.Done()` 監視 / Graceful Shutdown | ✅ `sync.WaitGroup` ＋ `ctx.Done()` の select で実装 |

---

### Phase 7: BigQuery クライアント

| # | タスク | 状態 |
|:---:|:---|:---:|
| 7-1 | GCP: BigQuery API 有効化 | ✅ |
| 7-2 | GCP: BigQuery データセット作成 | 🚧 **Terraform で定義済み**（`iac/bigquery.tf`）。`apply` 済みかは要確認 |
| 7-3 | `go get cloud.google.com/go/bigquery` | ✅ `v1.81.0` |
| 7-4 | `internal/bq/client.go` 作成 | ✅ `NewClient` / `InsertRows` / `Close` |

---

### Phase 8: ETL パイプライン統合 🔴 **ここが未達**

| # | タスク | 状態 |
|:---:|:---|:---:|
| 8-1 | `cmd/worker/main.go` 作成 | ❌ Issue #6（**Issue #63 の Firestore 移行完了後に着手**） |
| 8-2 | Drive → Postgres → Proto → BQ の End-to-End 疎通 | ❌ |
| 8-3 | Graceful Shutdown テスト | ❌ |

---

### IaC (Terraform)

| # | タスク | 状態 |
|:---:|:---|:---:|
| T-1 | `iac/main.tf` 作成 | ✅ |
| T-2 | `iac/bigquery.tf`（データセット / テーブル定義） | ✅ |
| T-3 | `iac/variables.tf` / `terraform.tfvars.example` | ✅ |

---

### 品質・セキュリティ（progress.md に項目が無かったので追加）

| # | タスク | 状態 |
|:---:|:---|:---:|
| Q-1 | ユニットテスト | 🚧 `internal/bq` / `internal/etl` のみ。`repository` / `drive` は未 |
| Q-2 | インターフェース＋モック生成（`make mock`） | ✅ 4パッケージ分を `mockgen` で生成 |
| Q-3 | CI（`.github/workflows/ci.yml`） | ✅ |
| Q-4 | OpenSSF Scorecard（`scorecard.yml`） | ✅ |
| Q-5 | Dependabot（`.github/dependabot.yml`） | ✅ |
| Q-6 | E2E 結合テスト（モックベース） | ❌ Issue #46 |

---

## 凡例

| 記号 | 意味 |
|:---:|:---|
| ✅ | 完了 |
| 🚧 | 着手済み・一部未完了 |
| ❌ | 未着手 |
| ❓ | 要確認 |
| 🐛 | バグあり |
