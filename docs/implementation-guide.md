# 実装ガイド

「1ファイルをエラーなく回収・検証し、BigQuery Raw層まで並行パイプラインで流す」MVP（最小限の構成）を段階的に作る。

---

## Phase 1: プロジェクト基盤

### Step 1-1: Go モジュール初期化

```sh
go mod init go-drive-etl
```

### Step 1-2: ディレクトリ作成 (Proto・GCP対応)

```sh
mkdir -p proto
mkdir -p infra
mkdir -p cmd/worker
mkdir -p cmd/auth
mkdir -p internal/drive
mkdir -p internal/parser
mkdir -p internal/etl
mkdir -p internal/repository
mkdir -p internal/bq
mkdir -p internal/pb
```

### Step 1-3: docker-compose.yml (状態管理用 Postgres)

```yml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: password
      POSTGRES_DB: app_metadata_db
    ports:
      - "5432:5432"
    volumes:
      - postgres_metadata_data:/var/lib/postgresql/data

volumes:
  postgres_metadata_data:
```

---

## Phase 2: スキーマ定義 (Protocol Buffers)

`proto/record.proto` を作成し、データパイプラインを流れるデータの型を厳密に定義する。

```proto
syntax = "proto3";

package pb;
option go_package = "go-drive-etl/internal/pb";

message FileRecord {
  string drive_file_id    = 1;
  string path             = 2;
  string mime_type        = 3;
  string checksum         = 4;
  string content_payload  = 5;
  int64  processed_at_unix = 6;
}
```

コンパイルコマンド:

```sh
protoc --go_out=. --go_opt=paths=source_relative proto/record.proto
```

---

## Phase 3: データベースマイグレーション (Metadata 用)

`migrations/001_init_metadata.sql` を作成する。

```sql
CREATE TABLE IF NOT EXISTS files (
  id            BIGSERIAL PRIMARY KEY,
  drive_file_id TEXT        NOT NULL UNIQUE,
  path          TEXT        NOT NULL,
  checksum      TEXT        NOT NULL,
  mime_type     TEXT        NOT NULL,
  sync_status   TEXT        NOT NULL DEFAULT 'pending',
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS jobs (
  id          BIGSERIAL PRIMARY KEY,
  status      TEXT        NOT NULL DEFAULT 'pending',
  retry_count INT         NOT NULL DEFAULT 0,
  started_at  TIMESTAMPTZ
);
```

---

## Phase 4: Repository (PostgreSQL)

既存の `internal/repository/file_repository.go` の実装をそのまま使用し、ファイルの状態管理・重複排除に専念させます。

---

## Phase 5: Google Drive クライアント

既存の `internal/drive/client.go` の実装をそのまま使用します。

---

## Phase 6: Worker Pool (Go Concurrency)

既存の `internal/etl/worker_pool.go` の実装をそのまま使用し、`ctx.Done()` 監視および Graceful Shutdown の設計を維持します。

---

## Phase 7: BigQuery クライアント (Proto 型セーフ版)

`internal/bq/client.go` を Protocol Buffers で生成された構造体をマッピングして流し込めるように修正します。

```go
package bq

import (
  "context"
  "fmt"

  "cloud.google.com/go/bigquery"
  "go-drive-etl/internal/pb"
)

type BQRecord struct {
  DriveFileID string `bigquery:"drive_file_id"`
  Path        string `bigquery:"path"`
  MimeType    string `bigquery:"mime_type"`
  Checksum    string `bigquery:"checksum"`
  Payload     string `bigquery:"content_payload"`
}

type Client struct {
  client    *bigquery.Client
  projectID string
  datasetID string
}

func NewClient(ctx context.Context, projectID, datasetID string) (*Client, error) {
  c, err := bigquery.NewClient(ctx, projectID)
  if err != nil {
    return nil, err
  }
  return &Client{client: c, projectID: projectID, datasetID: datasetID}, nil
}

func (c *Client) InsertProtoRecord(ctx context.Context, pbRec *pb.FileRecord) error {
  inserter := c.client.Dataset(c.datasetID).Table("raw_files").Inserter()

  record := BQRecord{
    DriveFileID: pbRec.DriveFileID,
    Path:        pbRec.Path,
    MimeType:    pbRec.MimeType,
    Checksum:    pbRec.Checksum,
    Payload:     pbRec.ContentPayload,
  }

  if err := inserter.Put(ctx, record); err != nil {
    return fmt.Errorf("bq streaming insert failed: %w", err)
  }
  return nil
}

func (c *Client) Close() error {
  return c.client.Close()
}
```

---

## Phase 8: ETL パイプライン統合 (main.go)

`cmd/worker/main.go` にて、Drive からデータを取得 → Postgres でステータス更新 → Proto によるバリデーション → BigQuery へ並行挿入の一連の流れを結合します。

---

## 実装順チェックリスト

- [ ] Step 1: `proto` ディレクトリ作成と `protoc` によるコンパイル環境構築
- [ ] Step 2: `infra` フォルダに Terraform ファイル（`main.tf`）を用意し、GCP の BigQuery データセットをコード定義
- [ ] Step 3: Docker Compose で Postgres の起動
- [ ] Step 4: Repository・Drive Client の繋ぎ込み
- [ ] Step 5: Worker Pool による複数ファイルダウンロードの並行処理の確認
- [ ] Step 6: パースデータを `pb.FileRecord` にのせ、BigQuery へ流し込むストリーミングインサートの実装
- [ ] Step 7: OS シグナルを送信した際の Graceful Shutdown テスト
