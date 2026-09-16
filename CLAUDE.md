## プロジェクト概要

Go + GCP (BigQuery) + Protocol Buffers の並行ETL / データパイプライン。
詳細は [docs/architecture.md](docs/architecture.md) 参照。

## 実装スタンス

- **ユーザーが自分でコーディングする**プロジェクト（学習目的）であり、Go の設計とコア実装はユーザーが担う。
- GitHub Issue を起点に自律実装してよいのは、**`agent-ok` ラベルが付いた Issue だけ**である。ラベルがない Issue では、コードを直接編集せず、手順・設計・スニペットを提示してガイドする。
- この会話でユーザーから「書いて」「実装して」と明示された場合は、その依頼の範囲で直接編集してよい。これは Issue 駆動の自律実装を許可するものではない。

### `agent-ok` の判断基準

次のうち、既存パターンに沿って完了条件を機械的に検証でき、主要な設計判断を含まないものは `agent-ok` にする。

- ドキュメント、CI、テスト追加、開発ツール設定、定型的な chore
- Terraform などの IaC のコード変更（`terraform apply`、本番リソース変更、state 移行は除く）
- BigQuery の SQL View / Mart と BI 向けの定義

次のものは学習効果を優先して `agent-ok` にしない。ユーザーが実装し、エージェントはレビューやガイドを担当する。

- Go の並行処理、パイプライン、parser、ETL、worker のコア設計・実装
- 新しい外部 API 統合、RAG/LLM の設計、データモデルや冪等性などの横断的な設計判断
- GCP コンソール操作、認証情報・Secret の操作、`terraform apply` など外部状態を変更する作業

`agent-ok` を付ける前に、依存 Issue が解消済みであること、受け入れ条件が具体的であること、上記の禁止作業を含まないことを確認する。ラベルは「自律実装の許可」を表すため、モデル選択ラベルとは独立して必ず付与する。

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
