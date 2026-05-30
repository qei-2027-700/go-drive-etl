# go-drive-etl

Go (Concurrency) + GCP (BigQuery) + Protocol Buffers を用いたバッチ ETL データパイプライン。

Google Drive にアップロードされた業務ファイルを自動回収し、BigQuery へロード。集計レポートを Drive へ書き戻す、実務直結型のデータエンジニアリングサイクルを実装する。

[![CI](https://github.com/qei-2027-700/go-drive-etl/actions/workflows/ci.yml/badge.svg)](https://github.com/qei-2027-700/go-drive-etl/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/qei-2027-700/go-drive-etl/badge)](https://securityscorecards.dev/viewer/?uri=github.com/qei-2027-700/go-drive-etl)

---

## アーキテクチャ

```
Google Drive (raw-inputs/)
    │
    ▼  Extract: ListFiles + メタデータ取得
PostgreSQL (Docker)
    │  状態管理・冪等性保証 (sync_status: pending → done)
    │
    ▼  Worker Pool (goroutine × 5並列)
    │
    ├─▶ BigQuery (etl_raw)        Bronze: 生データ保管
    │       ↓ SQL View
    │   BigQuery (Silver/Gold)    Transform → Analytics-ready
    │
    └─▶ Google Drive (export-reports/)  集計レポートを書き戻し
```

### メダリオンアーキテクチャ

| Layer | Component | 役割 |
|---|---|---|
| Bronze | BigQuery `etl_raw` | 生データをそのまま永続化 |
| Silver | BigQuery View | Protocol Buffers 定義の型安全な変換層 |
| Gold | BigQuery Mart | 可視化・レポート配信用の集計層 |
| Metadata | PostgreSQL | 処理ステータス・重複排除・リトライ管理 |

---

## 技術スタック

| カテゴリ | 技術 |
|---|---|
| 言語 | Go 1.26 |
| 並行処理 | goroutine / Worker Pool / `context.Context` |
| スキーマ管理 | Protocol Buffers |
| データソース | Google Drive API v3 (OAuth2) |
| DWH | BigQuery (GCP) |
| 状態管理DB | PostgreSQL (Docker) |
| IaC | Pulumi (Go) |
| CI/CD | GitHub Actions |
| セキュリティ | OSSF Scorecard / govulncheck / Dependabot |

---

## セットアップ

### 前提条件

- Go 1.26+
- Docker
- GCP プロジェクト（BigQuery API / Drive API 有効化済み）
- `gcloud` CLI + `bq` CLI

### 1. 認証情報の設定

```bash
cp .env.example .env
# .env に以下を設定:
# GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / GOOGLE_REFRESH_TOKEN
# BIGQUERY_PROJECT_ID / BIGQUERY_DATASET_ID
# DRIVE_FOLDER_ID / POSTGRES_DSN
```

リフレッシュトークンの取得:

```bash
go run ./cmd/auth/
# 表示された URL をブラウザで開いて認証
# ターミナルに表示された GOOGLE_REFRESH_TOKEN を .env に設定
```

BigQuery の ADC 設定:

```bash
gcloud auth application-default login
```

### 2. PostgreSQL 起動

```bash
docker compose up -d
```

### 3. 動作確認（Drive → PostgreSQL → BigQuery）

```bash
go run ./cmd/verify_drive/
```

---

## ディレクトリ構成

```
go-drive-etl/
├── cmd/
│   ├── auth/           # OAuth2 リフレッシュトークン取得ツール
│   ├── verify_drive/   # Drive→PostgreSQL→BigQuery 疎通確認ツール
│   └── worker/         # ETL パイプライン本体（実装予定）
├── internal/
│   ├── bq/             # BigQuery クライアント
│   ├── domain/         # ドメイン型定義
│   ├── drive/          # Google Drive クライアント
│   ├── etl/            # Worker Pool（実装予定）
│   ├── parser/         # CSV/JSON パーサー（実装予定）
│   ├── pb/             # Protocol Buffers 生成コード（実装予定）
│   └── repository/     # PostgreSQL リポジトリ
├── proto/              # Protocol Buffers 定義ファイル（実装予定）
├── migrations/         # DB マイグレーション SQL
├── iac/                # Pulumi IaC（実装予定）
└── docs/               # アーキテクチャ・設計ドキュメント
```

---

## 実装状況

| Phase | 内容 | 状態 |
|---|---|---|
| 1 | プロジェクト基盤・PostgreSQL | ✅ |
| 2 | Protocol Buffers スキーマ定義 | 🔲 |
| 3 | DB マイグレーション | ✅ |
| 4 | PostgreSQL Repository | ✅ |
| 5 | Google Drive クライアント | ✅ |
| 6 | Worker Pool (並行処理) | 🔲 |
| 7 | BigQuery クライアント | ✅ |
| 8 | ETL パイプライン統合 | 🔲 |
| IaC | Pulumi (GCP リソース管理) | 🔲 |
