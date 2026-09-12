# go-drive-etl

Go の並行処理 + GCP (BigQuery) + Protocol Buffers で構成した、バッチ ETL データパイプライン。

Google Drive に置かれた業務ファイルを自動回収し、スキーマ検証を通して BigQuery へロードする。集計結果は Drive へ CSV レポートとして書き戻し、蓄積したテキストチャンクは RAG の検索対象として再利用する。

[![CI](https://github.com/qei-2027-700/go-drive-etl/actions/workflows/ci.yml/badge.svg)](https://github.com/qei-2027-700/go-drive-etl/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/qei-2027-700/go-drive-etl/badge)](https://securityscorecards.dev/viewer/?uri=github.com/qei-2027-700/go-drive-etl)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)

---

## 3 分でわかる概要

| 問い | 答え |
|:---|:---|
| 何を解くのか | 非エンジニアが Drive に置く雑多なファイル（MD / CSV / JSON / PDF）を、人手を介さず分析可能な形へ落とし込む |
| なぜ Drive なのか | 現場との実運用インターフェースであり、「混沌とした外部ストレージからの回収」という実務課題をそのまま再現できる |
| 中心にある技術 | goroutine ベースの Worker Pool、Protocol Buffers によるスキーマ定義、Firestore による冪等性の担保 |
| どこまで動くか | Drive → 状態管理 DB → BigQuery の疎通は動作確認済み。通し実行のエントリポイント `cmd/worker` は実装中 |

---

## アーキテクチャ

```mermaid
flowchart LR
    subgraph SRC["Google Drive"]
        RAW["/raw-inputs/<br/>MD · CSV · JSON · PDF"]
        OUT["/export-reports/<br/>集計 CSV"]
    end

    subgraph GO["Go ETL Worker"]
        EX["Extract<br/>ListFiles + メタデータ"]
        DL["Download<br/>ストリーム取得"]
        PV["Parse & Validate<br/>Protocol Buffers 型検証"]
        LD["Load<br/>Streaming Insert"]
        WP["Worker Pool<br/>goroutine × 5"]
    end

    STATE[("Firestore<br/>状態管理・冪等性")]

    subgraph BQ["BigQuery"]
        BRONZE["Bronze / etl_raw<br/>生データ"]
        SILVER["Silver / View<br/>型安全な変換層"]
        GOLD["Gold / Mart<br/>集計層"]
    end

    subgraph CONS["活用"]
        LOOKER["Looker Studio<br/>ダッシュボード"]
        RAG["RAG Agent CLI<br/>Vertex AI Embeddings"]
    end

    RAW --> EX --> DL --> PV --> LD
    WP -.並行制御.-> DL
    EX <-->|checksum で重複排除| STATE
    LD --> STATE
    LD --> BRONZE --> SILVER --> GOLD
    GOLD --> OUT
    GOLD --> LOOKER
    SILVER --> RAG
```

### メダリオンアーキテクチャ

| Layer | Component | 役割 |
|:---|:---|:---|
| Bronze | BigQuery `etl_raw` | 取得した生データをそのまま永続化する |
| Silver | BigQuery View | Protocol Buffers 定義に沿った型安全な変換層。共通ロジックを View にカプセル化する |
| Gold | BigQuery Mart | 可視化・レポート配信のための最終集計層 |
| Metadata | Firestore | 処理ステータス・チェックサム・リトライ回数のみを保持する。データ実体は持たない。`STATE_BACKEND` で PostgreSQL に切り替えられる |

詳細な設計判断は [docs/architecture.md](docs/architecture.md) にまとめている。

---

## 設計上のポイント

### 1. Worker Pool による並行処理の制御

ファイル数に比例して goroutine を無制限に起動すると、Drive API のレートリミットと DB コネクションを容易に枯渇させる。バッファ付きチャネルをジョブキューとし、固定 5 並列のワーカーが取り出す形にしている。

```go
jobs := make(chan *domain.File, 100)

var wg sync.WaitGroup
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            select {
            case file, ok := <-jobs:
                if !ok {
                    return
                }
                // ダウンロード → パース → ロード
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

実装は [internal/etl/worker_pool.go](internal/etl/worker_pool.go)。

### 2. Graceful Shutdown

各ワーカーは `ctx.Done()` を select で監視する。`SIGTERM` 受信時は新規ジョブの取得を止め、`sync.WaitGroup` で処理中のジョブの完了を待ってから DB 接続を閉じる。ジョブ投入側も `ctx.Done()` を見ているため、キュー詰まりのままハングしない。

### 3. 冪等性

同じファイルを二重にロードしないよう、状態は状態管理 DB 側に寄せている。

- Drive の `drive_file_id` をそのままドキュメント ID に使い、`Upsert` で再実行を吸収する。Firestore は同じ ID への書き込みが上書きになるため、重複判定の分岐を書く必要がない（PostgreSQL 実装では `drive_file_id` の UNIQUE 制約と `ON CONFLICT` が同じ役割を担う）
- Drive が返す `md5Checksum` を保存している（保存までで、内容比較による再処理スキップは未実装）
- ステータスは `pending → processing → done / failed` と遷移し、失敗したファイルだけを次回拾い直せる

実装は [internal/repository/firestore_repository.go](internal/repository/firestore_repository.go)。PostgreSQL 側のスキーマは [migrations/001_init.sql](migrations/001_init.sql)。

### 4. Protocol Buffers によるスキーマ管理

パイプラインを流れるレコードの型は `.proto` を単一の情報源として定義し、Go の構造体を生成して使う。gRPC は使わず、スキーマ定義とバリデーションの用途に限定している。

```proto
message FileRecord {
  string drive_file_id = 1;
  string path = 2;
  string checksum = 3;
  string mime_type = 4;
  SyncStatus sync_status = 5;
  google.protobuf.Timestamp updated_at = 6;
}
```

定義は [proto/record.proto](proto/record.proto)、生成コードは `internal/pb/`。

### 5. テスト容易性

Drive / BigQuery / 状態管理 DB の各クライアントはインターフェース越しに扱い、`mockgen` でモックを生成している（`make mock`）。外部サービスに接続せずに Worker Pool のロジックを検証できる。

### 6. インフラと CI

BigQuery のデータセットとテーブルは Terraform で定義し、手作業の構成変更を残さない（[iac/](iac/)）。CI では gofmt 検証・`go vet`・`govulncheck`・テストを実行し、Dependabot と OpenSSF Scorecard で依存とリポジトリ設定を継続的に監視している。

---

## 技術スタック

| カテゴリ | 技術 |
|:---|:---|
| 言語 | Go 1.26 |
| 並行処理 | goroutine / Worker Pool / `context.Context` |
| スキーマ管理 | Protocol Buffers |
| データソース | Google Drive API v3（サービスアカウント / ADC） |
| DWH | BigQuery |
| BI / 可視化 | Looker Studio（Phase 3 で導入予定） |
| 状態管理 DB | Firestore（既定）/ PostgreSQL 16 (Docker) |
| IaC | Terraform (Google Provider ~> 6.0) |
| テスト | `go test` / `mockgen` |
| CI/CD | GitHub Actions |
| セキュリティ | OpenSSF Scorecard / govulncheck / Dependabot |

---

## セットアップ

### 前提条件

- Go 1.26 以上
- Docker
- GCP プロジェクト（BigQuery API / Drive API 有効化済み）
- `gcloud` CLI

### 1. git フックの有効化

コミット時にステージ済みの Go ファイルへ自動で `gofmt` を適用する。`core.hooksPath` はリポジトリに含められないため、クローンごとに 1 度実行する。

```bash
git config core.hooksPath .githooks
```

`git add -p` で一部だけをステージしている場合は、ステージ外の変更を巻き込まないよう自動整形せずコミットを中止する。その場合は `gofmt -w <file>` を実行し、整形結果をステージし直す。

### 2. 認証情報の設定

```bash
cp .env.example .env
```

`.env` に以下を設定する。

| 変数 | 用途 |
|:---|:---|
| `GOOGLE_APPLICATION_CREDENTIALS` | Drive API の認証に使うサービスアカウントの JSON キーへのパス |
| `BIGQUERY_PROJECT_ID` / `BIGQUERY_DATASET_ID` | ロード先の BigQuery |
| `DRIVE_FOLDER_ID` | 回収対象の Drive フォルダ |
| `STATE_BACKEND` | 状態管理のバックエンド。`postgres` または `firestore`（省略時は `firestore`） |
| `GOOGLE_CLOUD_PROJECT` | Firestore を使う場合の GCP プロジェクト |
| `FIRESTORE_EMULATOR_HOST` | ローカルでエミュレータを使う場合のみ設定する（例: `localhost:8080`）。本番の Firestore に接続するときは必ず空にする |
| `POSTGRES_DSN` | PostgreSQL を使う場合の接続文字列 |

#### Drive API: サービスアカウントの準備

Drive API は OAuth2 のユーザー委譲ではなく、サービスアカウントで認証する。OAuth ユーザー委譲は、同意画面が「テスト中」ステータスの間リフレッシュトークンが 7 日で失効し、常時稼働に向かないため採用していない（経緯は [Issue #73](https://github.com/qei-2027-700/go-drive-etl/issues/73) を参照）。

1. GCP コンソールでサービスアカウントを作成する（例: `etl-worker`）
2. 「キー」タブから JSON キーを作成してダウンロードする。**JSON キーはパスワード同等の機密情報なので、リポジトリ外に保存し `.gitignore` で除外すること**
3. 対象の Drive フォルダ（`DRIVE_FOLDER_ID`）を、サービスアカウントのメールアドレス（`<name>@<project-id>.iam.gserviceaccount.com`）に**閲覧者権限で共有する**。サービスアカウントは独立した利用者のため、共有を忘れるとファイルが 1 件も見えない
4. ダウンロードした JSON キーのパスを `.env` の `GOOGLE_APPLICATION_CREDENTIALS` に設定する

BigQuery / Firestore は引き続き ADC（Application Default Credentials）で認証する。

```bash
gcloud auth application-default login
```

> **OAuth2 ユーザー委譲方式（フォールバック）**
> サービスアカウント方式が使えない環境向けに、コードと手順は残してある。`.env.example` の `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_REFRESH_TOKEN` のコメントを外し、`go run ./cmd/auth/` でリフレッシュトークンを取得する。有効化する場合は `internal/drive/client.go` の `NewClient` も OAuth 版に戻す必要がある（同ファイルのコメントに手順を記載）。通常は使わない。

### 3. 状態管理 DB の起動

状態管理は Firestore と PostgreSQL のどちらでも動く。`STATE_BACKEND` で切り替える。

**Firestore（既定）** — ローカルではエミュレータを使う。GCP の認証も課金も発生しない。

```bash
docker compose up -d firestore
# .env で FIRESTORE_EMULATOR_HOST=localhost:8080 を有効にする
```

本番の Firestore に向ける場合はエミュレータを起動せず、`FIRESTORE_EMULATOR_HOST` を空にしたうえで `GOOGLE_CLOUD_PROJECT` を設定する。データベース自体は次の手順の Terraform で作成される。

**PostgreSQL** — `.env` に `STATE_BACKEND=postgres` を設定する。

```bash
docker compose up -d postgres
docker compose exec -T postgres psql -U app -d app_db < migrations/001_init.sql
```

### 4. インフラの適用

```bash
cd iac
cp terraform.tfvars.example terraform.tfvars  # project_id などを設定
terraform init && terraform apply
```

BigQuery のデータセット / テーブルに加え、Firestore データベース（`(default)`）と Firestore API の有効化が適用される。

---

## デモ

Drive → 状態管理 DB → BigQuery の疎通を 1 コマンドで確認できる。

```bash
go run ./cmd/verify_drive/
```

```txt
Drive 取得ファイル数: 3

  ✓ 状態管理DB Upsert: 2026-W35-retrospective.md
  ✓ 状態管理DB Upsert: sales_2026q2.csv
  ✓ 状態管理DB Upsert: meeting-notes.pdf
  状態管理DB pending 件数: 3

  ✓ BigQuery Insert: 3 件

--- Drive → 状態管理DB → BigQuery 疎通完了 ---
```

テストとモックの生成は Make 経由で行う。

```bash
go test ./...
make mock
```

リポジトリ層のテストは実物のバックエンドに対して実行する。環境変数が未設定なら自動でスキップされるため、`go test ./...` はそのままでも通る。

```bash
# Firestore（エミュレータ）
docker compose up -d firestore
FIRESTORE_EMULATOR_HOST=localhost:8080 go test ./internal/repository/

# PostgreSQL（files テーブルを空にする）
docker compose up -d postgres
POSTGRES_TEST_DSN=postgres://app:password@localhost:5432/app_db?sslmode=disable \
  go test ./internal/repository/
```

> Looker Studio ダッシュボードと RAG Agent CLI の実行例は、該当フェーズの実装完了後に追記する。

---

## ディレクトリ構成

```txt
go-drive-etl/
├── cmd/
│   ├── auth/           # OAuth2 リフレッシュトークン取得ツール（フォールバック用、通常は未使用）
│   ├── verify_drive/   # Drive → 状態管理 DB → BigQuery 疎通確認ツール
│   └── worker/         # ETL パイプライン本体（実装中）
├── internal/
│   ├── bq/             # BigQuery クライアント
│   ├── domain/         # ドメイン型定義
│   ├── drive/          # Google Drive クライアント
│   ├── etl/            # Worker Pool
│   ├── parser/         # ファイルパーサー（実装中）
│   ├── pb/             # Protocol Buffers 生成コード
│   └── repository/     # 状態管理リポジトリ（Firestore / PostgreSQL）
├── proto/              # Protocol Buffers 定義
├── migrations/         # PostgreSQL マイグレーション SQL
├── iac/                # Terraform（BigQuery / Firestore）
└── docs/               # アーキテクチャ・設計ドキュメント
```

---

## 実装状況

| 領域 | 状態 |
|:---|:---:|
| プロジェクト基盤 / 状態管理 DB | ✅ |
| Protocol Buffers スキーマ定義 | ✅ |
| DB マイグレーション | ✅ |
| 状態管理 Repository（Firestore / PostgreSQL） | ✅ |
| Google Drive クライアント | ✅ |
| Worker Pool（並行処理） | ✅ |
| BigQuery クライアント | ✅ |
| Terraform（BigQuery / Firestore） | ✅ |
| CI / セキュリティ監視 | ✅ |
| ファイルパーサー（チャンク化） | 🚧 |
| ETL パイプライン統合（`cmd/worker`） | 🚧 |

進捗の内訳は [docs/progress.md](docs/progress.md) を参照。

---

## ロードマップ

| Phase | ゴール |
|:---|:---|
| Phase 1 | 任意のファイルを Drive から取り込み、チャンク単位で BigQuery に格納するまでを通しで動かす |
| Phase 2 | Vertex AI Embeddings と BigQuery Vector Search を用いた RAG Agent CLI を構築し、Recall / Faithfulness を計測する |
| Phase 3 | Gold 層を Looker Studio で可視化し、集計 CSV を Drive へ自動デリバリーする |

---

## ドキュメント

| ファイル | 内容 |
|:---|:---|
| [docs/architecture.md](docs/architecture.md) | レイヤー設計・Worker の責務・ロードマップの詳細 |
| [docs/implementation-guide.md](docs/implementation-guide.md) | 実装手順 |
| [docs/local-development.md](docs/local-development.md) | ローカル開発環境 |
| [docs/security.md](docs/security.md) | セキュリティ方針 |
| [docs/progress.md](docs/progress.md) | フェーズ別の進捗 |
