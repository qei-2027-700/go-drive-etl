# go-drive-etl

Google Drive → Go ETL Worker → PostgreSQL → BigQuery のデータパイプライン。

転職・学習・ポートフォリオ用途として、Go concurrency / ETL / GCP を実践的に学ぶためのプロジェクト。

## アーキテクチャ

```text
Google Drive  (Bronze: Raw Data Lake)
      ↓
Go Worker     (ETL / Sync Engine)
      ↓
PostgreSQL    (Silver: 状態管理DB)
      ↓
BigQuery      (Gold: Analytics / AI-ready DWH)
      ↓
Analytics / AI / RAG
```

## ドキュメント

- [アーキテクチャ詳細](docs/architecture.md)
- [実装ガイド](docs/implementation-guide.md)

## Quick Start

```bash
# PostgreSQL 起動
docker compose up -d

# Worker 実行
go run ./cmd/worker
```

## 将来構成

```text
Cloud Scheduler → Cloud Run Worker → Drive API → Postgres → BigQuery
```


```sh
psql postgres://app:password@localhost:5432/app_db -f migrations/001_init.sql
```

```sh
psql postgres://app:password@localhost:5432/app_db -c "\dt"
```

```sh
# 実行用コマンド
GOOGLE_CLIENT_ID="" GOOGLE_CLIENT_SECRET="" go run ./cmd/auth
```