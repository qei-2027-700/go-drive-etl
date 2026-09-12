# アーキテクチャ詳細

---

## データレイヤー (メダリオン・アーキテクチャ)

| Layer    | Component                                          | Role                  | Description                                                                                                                       |
| :------- | :------------------------------------------------- | :-------------------- | :-------------------------------------------------------------------------------------------------------------------------------- |
| —        | Google Drive                                       | 外部データソース      | 現場がファイルをアップロードする最上流。入口であり、出口でもある。                                                                |
| Bronze   | Cloud Storage (GCS) / BigQuery (Raw データセット)  | Raw Data Lake         | 取得した生ファイル、または未加工のデータをそのまま永続化する層。                                                                  |
| Silver   | BigQuery (Component / Warehouse 層)                | Trusted Layer         | Protocol Buffers でスキーマ定義された型安全な構造。SQL View を用いて共通ビジネスロジックをカプセル化（コンポーネント化）。        |
| Gold     | BigQuery (Mart 層)                                 | Analytics-ready DWH   | 可視化（Looker Studio）や Google Drive への CSV レポート自動デリバリー用に最適化された最終集計層。                                |
| Metadata | Firestore                                          | 状態管理 DB           | ファイルの処理ステータス、チェックサム（重複排除）、ジョブのリトライ管理などの「ステート（状態）」のみを管理。データの実体は保持しない。`STATE_BACKEND` で PostgreSQL 実装に切り替えられる。 |

---

## Google Drive のディレクトリ構成

```
/raw-inputs/       # 入力：現場が格納する未加工ファイル (PDF, CSV, JSON, MD)
/export-reports/   # 出力：パイプラインが最終出力する集計 CSV レポート
```

> **Drive を使う理由**：非エンジニアの現場との最高のエンドツーエンドのインターフェースになるため。実務で頻出する「混沌とした外部ファイルストレージからのクレンジング回収」を再現するため。

---

## 状態管理のデータ構造

状態管理は `FileRepo` インターフェース越しに扱い、Firestore と PostgreSQL の 2 実装を `STATE_BACKEND` で切り替える。既定は Firestore。

### Firestore（既定）

コレクションは `files` ひとつ。**ドキュメント ID に Drive の `drive_file_id` をそのまま使う**のが設計の核心で、これにより冪等性が構造として保証される。同じファイルを再取得しても同じドキュメントを指すため、書き込みは常に上書きになり、重複判定のロジックを書く必要がない。

```txt
files/{drive_file_id}
  drive_file_id : string     // ドキュメント ID と同じ値を冗長に保持（クエリ結果から復元するため）
  path          : string
  checksum      : string     // Drive が返す md5Checksum
  mime_type     : string
  sync_status   : string     // pending | processing | done | failed
  updated_at    : timestamp  // serverTimestamp。クライアントの時計に依存させない
```

未処理ファイルの抽出は `sync_status == "pending"` の単一フィールド等価検索で行う。自動インデックスで足りるため、複合インデックスの定義は不要。

Firestore に連番の整数 ID は存在しないため、`domain.File` は数値 ID を持たず、`drive_file_id`（string）を識別子とする。

### PostgreSQL（切り替え時）

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

Firestore の「ドキュメント ID による上書き」に相当するのが `drive_file_id` の UNIQUE 制約と `ON CONFLICT ... DO UPDATE` で、役割は同じ。

### 挙動の差分

`UPDATE ... WHERE` は対象行が無くても no-op だが、Firestore の `Update` は存在しないドキュメントに対して `NotFound` を返す。呼び出し側は未処理一覧から得た ID しか渡さないため、このエラーは実際の不整合を示す。

---

## Go Worker の責務

| ステップ             | 内容                                                                                                                    |
| :------------------- | :---------------------------------------------------------------------------------------------------------------------- |
| **Extract**          | Drive API から新着ファイルを検知（状態管理 DB の `checksum` で重複排除）                                              |
| **Download**         | ファイルをストリームでローカルに取得                                                                                    |
| **Parse & Validate** | Go の Parser（CSV/JSON）で分解し、Protocol Buffers から自動生成された Go 構造体（Struct）にマッピングしてスキーマ検証  |
| **Load (Bronze)**    | スキーマ整合性の取れたデータを BigQuery Raw 層へ並行高速ロード（Streaming Insert / Bulk Load）                         |
| **State Update**     | 状態管理 DB のステータスを `done` に更新                                                                                |
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

## IaC (Terraform)

インフラは **Terraform（Google Provider ~> 6.0）** で管理する。BigQuery まわりのリソース定義が宣言的に読め、GCP のドキュメントやサンプルもそのまま流用できる。

### 管理対象リソース

| リソース | 内容 |
| :--- | :--- |
| BigQuery dataset | `etl_raw`（Bronze 層）。課金未設定のためテーブル・パーティションに 60 日の有効期限を設定 |
| BigQuery table | `etl_raw.drive_files`（スキーマ定義） |
| （将来）IAM | BI ツール / Worker 用サービスアカウント + 最小権限ロール付与 |
| （将来）GCS bucket | 生ファイルの保管用 Bronze 層 |

### ディレクトリ構成

```
iac/
├── main.tf                    # provider / terraform ブロック
├── bigquery.tf                # dataset・table 定義
├── variables.tf               # project_id / dataset_id / location
├── terraform.tfvars           # 実値（Git 管理外）
└── terraform.tfvars.example   # 雛形
```

### 方針

- ステートはローカルの `terraform.tfstate`。複数環境を扱う段階で GCS バックエンドへ移す
- 環境は `dev` のみで開始し、将来 `prod` を追加
- ADC（Application Default Credentials）で認証（ローカル: `gcloud auth application-default login`）

---

## ロードマップ

### Phase 1 — AI-Ready Pipeline（現在進行中）

Drive に置いた Notion 週次振り返り MD を取り込み、BigQuery にチャンク単位で格納するところまでを完成させる。

```
任意のファイル（MD / PDF / CSV / テキスト等）を手動アップロード
  例: Notion 週次振り返り MD、議事録 PDF、データ CSV など
    ↓
Google Drive /raw-inputs/
    ↓ go-drive-etl スキャン
Go Worker（ETL）
    ├─ Firestore（状態管理・重複排除）
    └─ BigQuery
         ├─ Raw 層: ファイルメタデータ
         └─ Chunk 層: ChunkRecord（content, chunk_index）← ここまでが Phase 1
```

**Phase 1 の完了条件**
- [ ] ファイルパーサー実装（MD を優先、将来 PDF / CSV に拡張可能な設計）
- [ ] `ChunkRecord` を BigQuery に格納するテーブル・ロジック実装
- [ ] `cmd/worker` エントリーポイント完成（End-to-End 動作）
- [ ] Terraform で BQ テーブルを IaC 管理
- [ ] `go test ./...` が通る状態を維持

---

### Phase 2 — RAG Agent（Phase 1 完了後）

BigQuery に蓄積したチャンクを Vertex AI で埋め込み、Vector Search で検索可能にした上で、週次振り返りを参照できる RAG エージェントを構築する。データは GCP 内でクローズドに処理し、個人情報を外部 API に送出しない。

```
BigQuery Chunk 層
    ↓
Vertex AI Embeddings（GCP 内。外部 API 不使用）
    ↓
BigQuery Vector Search
    ↓
RAG Agent CLI
    ↓
LLMOps（評価・モデルバージョン管理）
```

**Phase 2 の完了条件**
- [ ] Vertex AI Embeddings でチャンク埋め込みパイプライン実装
- [ ] BigQuery Vector Search でセマンティック検索
- [ ] 週次振り返りを参照できる RAG Agent CLI 動作
- [ ] RAG 評価指標（Recall / Faithfulness）の計測

---

### Phase 3 — BI & Delivery（Phase 1 完了後）

Bronze に溜めたデータを Silver / Gold へ整形し、Looker Studio で可視化しつつ、集計 CSV を Drive へ書き戻す。「AI Ready」だけでなく「BI Ready」でもあることを示すフェーズ。

```
BigQuery Raw 層（Bronze）
    ↓ SQL View（共通ロジックのカプセル化）
BigQuery Silver 層
    ↓ 集計クエリ
BigQuery Mart 層（Gold）
    ├─▶ Looker Studio（BigQuery ネイティブコネクタでダッシュボード）
    └─▶ Go Worker → CSV 化 → Google Drive /export-reports/
```

**BI ツールに Looker Studio を選ぶ理由**

- BigQuery のネイティブコネクタで接続でき、追加のサーバー運用が発生しない（Redash はセルフホスト前提で、本体 + PostgreSQL + Redis + ワーカーの運用が必要になる）
- Drive / BigQuery / IAM と同じ Google エコシステム内で認証・権限を統一できる
- 共有リンクでダッシュボードを公開でき、ポートフォリオとして提示しやすい

ダッシュボード定義は IaC 管理の対象外になるため、SQL ロジックは BigQuery の View 側に寄せて可視化層から独立させる。

**Phase 3 の完了条件**
- [ ] Silver 層の SQL View と Gold 層の Mart テーブルを設計・作成
- [ ] BI 接続用サービスアカウントを最小権限で Terraform 管理
- [ ] Looker Studio から Gold 層をクエリし、ダッシュボード 1 枚完成（README にスクリーンショット掲載）
- [ ] Gold 層の集計結果を CSV 化し Drive `/export-reports/` へ自動デリバリー
