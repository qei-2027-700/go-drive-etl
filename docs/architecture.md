# アーキテクチャ詳細

---

## データレイヤー (メダリオン・アーキテクチャ)

| Layer    | Component                                          | Role                  | Description                                                                                                                       |
| :------- | :------------------------------------------------- | :-------------------- | :-------------------------------------------------------------------------------------------------------------------------------- |
| —        | Google Drive                                       | 外部データソース      | 現場がファイルをアップロードする最上流。入口であり、出口でもある。                                                                |
| Bronze   | Cloud Storage (GCS) / BigQuery (Raw データセット)  | Raw Data Lake         | 取得した生ファイル、または未加工のデータをそのまま永続化する層。                                                                  |
| Silver   | BigQuery (Component / Warehouse 層)                | Trusted Layer         | Protocol Buffers でスキーマ定義された型安全な構造。SQL View を用いて共通ビジネスロジックをカプセル化（コンポーネント化）。        |
| Gold     | BigQuery (Mart 層)                                 | Analytics-ready DWH   | 可視化（Redash）や Google Drive への CSV レポート自動デリバリー用に最適化された最終集計層。                                       |
| Metadata | PostgreSQL                                         | 状態管理 DB           | ファイルの処理ステータス、チェックサム（重複排除）、ジョブのリトライ管理などの「ステート（状態）」のみを管理。データの実体は保持しない。 |

---

## Google Drive のディレクトリ構成

```
/raw-inputs/       # 入力：現場が格納する未加工ファイル (PDF, CSV, JSON, MD)
/export-reports/   # 出力：パイプラインが最終出力する集計 CSV レポート
```

> **Drive を使う理由**：非エンジニアの現場との最高のエンドツーエンドのインターフェースになるため。実務で頻出する「混沌とした外部ファイルストレージからのクレンジング回収」を再現するため。

---

## PostgreSQL スキーマ (状態管理の役割)

```sql
-- ファイル同期状態・冪等性 (Idempotency) の管理
CREATE TABLE IF NOT EXISTS files (
  id            BIGSERIAL PRIMARY KEY,
  drive_file_id TEXT        NOT NULL UNIQUE,
  path          TEXT        NOT NULL,
  checksum      TEXT        NOT NULL,
  mime_type     TEXT        NOT NULL,
  sync_status   TEXT        NOT NULL DEFAULT 'pending',
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ジョブキュー / リトライ管理 (BigQuery が苦手なトランザクション更新を肩代わり)
CREATE TABLE IF NOT EXISTS jobs (
  id          BIGSERIAL PRIMARY KEY,
  status      TEXT        NOT NULL DEFAULT 'pending',
  retry_count INT         NOT NULL DEFAULT 0,
  started_at  TIMESTAMPTZ
);
```

---

## Go Worker の責務

| ステップ             | 内容                                                                                                                    |
| :------------------- | :---------------------------------------------------------------------------------------------------------------------- |
| **Extract**          | Drive API から新着ファイルを検知（PostgreSQL の `checksum` で重複排除）                                                |
| **Download**         | ファイルをストリームでローカルに取得                                                                                    |
| **Parse & Validate** | Go の Parser（CSV/JSON）で分解し、Protocol Buffers から自動生成された Go 構造体（Struct）にマッピングしてスキーマ検証  |
| **Load (Bronze)**    | スキーマ整合性の取れたデータを BigQuery Raw 層へ並行高速ロード（Streaming Insert / Bulk Load）                         |
| **State Update**     | PostgreSQL の管理ステータスを `done` に更新                                                                             |
| **Export (Gold)**    | BigQuery Mart 層のデータを吸い上げ、CSV 化して Google Drive（`/export-reports/`）へ自動書き戻し                        |

---

## Worker Pool (並行処理制御)

無限 goroutine は避け、Worker Pool（デフォルト **5 並列**）によって API レートリミットや DB コネクション、メモリ爆発を防ぐ。

```
jobs channel (buffer=100)
        ↓
worker1 ~ worker5  (最大 5 並列で並行ダウンロード＆パース)
        ↓
BigQuery / GCS へロード
```

---

## `context.Context` による Graceful Shutdown

OS シグナル（`Ctrl+C` / `SIGTERM`）を検知すると、`context` がキャンセルされ、Worker Pool は現在のジョブをキープ、または安全に区切りの良いところで停止し、DB 接続をクローズして安全にシャットダウンする。

---

## IaC (Pulumi / Go)

インフラは **Pulumi (Go)** で管理する。ETL パイプラインと同じ Go で記述できるため、言語を統一しコンテキストスイッチを減らす。

### 管理対象リソース

| リソース | 内容 |
| :--- | :--- |
| BigQuery dataset | `etl_raw`（Bronze 層） |
| BigQuery table | `etl_raw.files`（スキーマ定義） |
| IAM | サービスアカウント + 最小権限ロール付与 |
| （将来）GCS bucket | 生ファイルの保管用 Bronze 層 |

### ディレクトリ構成

```
iac/
├── main.go         # Pulumi プログラムエントリーポイント
├── go.mod          # iac 専用モジュール（パイプライン本体と分離）
├── Pulumi.yaml     # プロジェクト定義
└── Pulumi.dev.yaml # dev スタックの設定値
```

### 方針

- `iac/` は独立した Go モジュールとして管理（`go.mod` を分ける）
- スタックは `dev` のみで開始し、将来 `prod` を追加
- ADC（Application Default Credentials）で認証（ローカル: `gcloud auth application-default login`）

---

## 将来の AI Ready 構成 (拡張ロードマップ)

```
Drive → Go Worker → Chunking → Embedding (Proto/gRPC) → BigQuery Vector Search → RAG / LLM
```
