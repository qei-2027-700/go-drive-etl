# ローカル開発環境セットアップ

## 前提条件

- Go 1.22+
- Docker（PostgreSQL 用）
- Google Cloud CLI (`gcloud`)
- Terraform

## GCP 認証

このプロジェクトは 2 種類の GCP 認証を使い分けます。

### CLI 操作用（gcloud コマンド）

```bash
gcloud auth login
```

- `gcloud` コマンド自体の認証
- `gcloud projects list` や `gcloud config` などの操作に使う

### アプリケーション認証（ADC）

```bash
gcloud auth application-default login
```

- Go コード（`bq/client.go`）および Terraform が使う認証
- `~/.config/gcloud/application_default_credentials.json` に保存される
- **ローカル開発では必ずこちらも実行すること**

### プロジェクト設定

```bash
gcloud config set project go-drive-etl
```

## 環境変数

`.env` ファイルをリポジトリルートに作成する（git 管理外）。

```env
# BigQuery
BIGQUERY_PROJECT_ID=go-drive-etl
BIGQUERY_DATASET_ID=etl_raw

# Google Drive OAuth2
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REFRESH_TOKEN=...
```

OAuth2 トークンの取得方法は `cmd/auth/` を参照。

## Terraform（IaC）

BQ データセット・テーブルの作成・変更は Terraform で管理する。

```bash
cd iac

# 初回のみ
terraform init

# 変更内容の確認
terraform plan

# 適用
terraform apply
```

`iac/terraform.tfvars` に実際の値を設定する（git 管理外）。`iac/terraform.tfvars.example` を参考に作成すること。

## PostgreSQL（メタデータ管理）

ファイルの処理状態は PostgreSQL で管理する。

```bash
# Docker で起動
docker compose up -d
```

マイグレーションの実行方法は未整備のため、今後 `docs/architecture.md` に追記予定。
