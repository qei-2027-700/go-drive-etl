## プロジェクト概要

Go + GCP (BigQuery) + Protocol Buffers の並行ETL / データパイプライン。
詳細は [docs/architecture.md](docs/architecture.md) 参照。

## 実装スタンス

- **ユーザーが自分でコーディングする**プロジェクト（学習目的）
- Claude はコードを直接書かず、**手順・スニペットを提示してガイド**する
- ユーザーが「書いて」「実装して」と明示したときのみ直接編集する

## 設計方針

- 厳密な Clean Architecture にはしない（ETL は pipeline / orchestration / stream が主役）
- スキーマ管理に **Protocol Buffers** を使用し、データの型やバリデーションルールを厳密に定義する
- 優先度: Pipeline > Schema Validation (Proto) > Retry > Idempotency > Worker Pool > Context Cancellation

## ディレクトリ構成

```txt
/proto             # Protocol Buffers 定義ファイル (.proto)
/infra             # Terraform (BigQuery, GCS, IAM などのインフラ定義)
/cmd/worker        # エントリーポイント (ETLパイプライン実行用)
/cmd/auth          # Google API 認証用
/internal/drive    # Drive API クライアント (ダウンロード / エクスポート用)
/internal/parser   # ファイルパーサー (PDF, CSV, JSON, MD) を並行処理でパース
/internal/etl      # ETL パイプライン・オーケストレーション
/internal/repository # PostgreSQL リポジトリ (ファイル処理のメタデータ・状態管理用)
/internal/bq       # BigQuery クライアント (Rawデータ挿入用)
/internal/pb       # Protocol Buffers から自動生成されたGoコード
```
