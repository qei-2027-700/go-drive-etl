# ローカル開発環境セットアップ

## 前提条件

- Go 1.26 以上
- Docker（Firestore エミュレータ / PostgreSQL 用）
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

初回だけ、最後に `apply` した state を指定して次を実行する。

```bash
GO_DRIVE_ETL_LEGACY_TF_STATE_PATH=/absolute/path/to/terraform.tfstate \
GO_DRIVE_ETL_LEGACY_TFVARS_PATH=/absolute/path/to/terraform.tfvars \
  ./scripts/terraform migrate-local-state
```

以後、Terraform を直接実行せず、全 worktree で共通のローカル state を使う
ラッパーを実行する。

```bash
# 初期化状態の確認
./scripts/terraform init

# 変更内容の確認
./scripts/terraform plan

# 適用
./scripts/terraform apply
```

既定の共有先は `~/.local/state/go-drive-etl/terraform.tfstate` と
`~/.config/go-drive-etl/terraform.tfvars`。state 未移行時はラッパーが失敗するため、
worktree が空の state で既存リソースを作成しようとすることを防げる。
保存先を変える場合は `GO_DRIVE_ETL_TF_STATE_DIR`、
`GO_DRIVE_ETL_TF_CONFIG_DIR`、`GO_DRIVE_ETL_TFVARS_PATH` を設定する。
`plan` では既存リソースが `create` / `destroy` と表示されないことを確認する。設定を
変更した場合の `update` は、state の欠落とは別にレビューする。

## 状態管理 DB

ファイルの処理状態は Firestore で管理する。`STATE_BACKEND` を `postgres` にすると PostgreSQL 実装に切り替わる（省略時は Firestore）。

### Firestore（既定）

ローカルではエミュレータを使う。GCP の認証も課金も発生しない。

```bash
docker compose up -d firestore
```

`.env` で `FIRESTORE_EMULATOR_HOST=localhost:8080` を有効にすると、クライアントライブラリが自動でエミュレータへ向く。Go 側に分岐は無い。

**本番の Firestore に接続するときは `FIRESTORE_EMULATOR_HOST` を必ず空にすること。** 値が残っていると、本番のつもりの実行が黙って localhost を見る。

リポジトリ層のテストもエミュレータに対して実行する。環境変数が未設定なら自動でスキップされるため、CI には影響しない。

```bash
FIRESTORE_EMULATOR_HOST=localhost:8080 go test ./internal/repository/
```

### PostgreSQL

`.env` に `STATE_BACKEND=postgres` を設定する。

```bash
# Docker で起動
docker compose up -d postgres

# マイグレーション適用
docker compose exec -T postgres psql -U app -d app_db < migrations/001_init.sql
```

接続情報は `docker-compose.yml` の値（ユーザー `app` / DB `app_db`）に対応する。

PostgreSQL 実装のテストも同じ形で実行する。`POSTGRES_TEST_DSN` が未設定なら自動でスキップされる。

```bash
POSTGRES_TEST_DSN=postgres://app:password@localhost:5432/app_db?sslmode=disable \
  go test ./internal/repository/
```

**このテストは `files` テーブルを空にする。** アプリが読む `POSTGRES_DSN` ではなく専用の変数を使うのは、接続先を取り違えて本番相当のデータを消さないようにするため。
