# go-drive-etl

Go (Concurrency) + GCP (BigQuery/GCS) + Protocol Buffers を用いた高性能バッチETLデータパイプライン。

転職・学習・ポートフォリオ用途として、Goの並行処理、スキーマ管理、モダンなデータレイク/DWH（データウェアハウス）設計を実戦的に学ぶためのプロジェクト。

## プロジェクトの背景・目的
他部署の非エンジニアメンバーや外部システムが、日々Google Driveにアップロードする未加工の業務データ（CSV, JSON, PDF等）を自動で回収・クレンジングし、データ分析基盤へと統合。
さらに集計された高度なビジネスレポートを再度Google Driveへ自動エクスポートして現場に還元する、実務直結型のデータエンジニアリング・サイクルを実装する。

## アーキテクチャ (データフロー)

```text
[Source] Google Drive (PDF / CSV / JSON / MD)
    ↓
[Extract] Go Worker (goroutine / Worker Pool)
    ├─ (状態管理 / 冪等性の担保) ──> PostgreSQL (Docker)
    ↓ 
[Load Raw] Cloud Storage (GCS) または BigQuery Raw層  (Bronze: Raw Data Lake)
    ↓ (Schema defined by Protocol Buffers)
[Transform] BigQuery SQL / View  (Silver: Warehouse / Component層)
    ↓
[Serving] BigQuery Mart層  (Gold: Analytics-ready)
    ├─> Redash (可視化・ダッシュボード接続)
    └─> [Export] Google Drive (CSVレポートとして指定フォルダへ自動格納)
