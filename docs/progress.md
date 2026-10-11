# 進捗管理

> GCP プロジェクト: **`go-drive-etl`**
> 最終更新: **2026-10-11**

## 現在地

Portfolio v1 の対象である Phase 1（Markdown を対象とする AI-Ready Pipeline）と Phase 3（BI & Delivery）の実装は完了している。Google Drive からの取得、Firestore による冪等管理、Markdown のチャンク化、BigQuery へのロード、Gold Mart の CSV 配信、データポータルでの可視化までを備える。`go test ./...` は全パッケージ成功。2026-10-11 のWorker実行で3件を検出し、追加ファイルを含む15チャンクをBigQueryへロードした。Gold Martを再集計し、DriveへCSVを再出力した。データポータルの [Ingestion health](https://datastudio.google.com/u/1/reporting/4ea2ca0b-3c98-4184-93d8-e779652451e2/page/AF69F) の最新値は再読み込み後に確認する。詳細は [README のデモ](../README.md#デモ) に記録している。

現在の未完了の**機能開発**は、RAG を完成させる Phase 2 の Issue 群だけである。依存順は **#36 → #37 → #38 → #39**（親: #23）。Protobuf 再生成の手順整備は、機能開発とは別の開発基盤タスクとして残っている。

| Issue | 内容 | 状態 |
|---:|---|:---:|
| #36 | Vertex AI Embeddings パイプライン | 未着手 |
| #37 | BigQuery Vector Search | #36 待ち |
| #38 | 週次振り返り RAG Agent CLI | #37 待ち |
| #39 | Recall / Faithfulness 評価 | #38 待ち |

> Vertex AI を実際に呼び出す #36 と #38 は課金対象になり得る。設計・インターフェース・モックテストを先に実装し、クラウド実行確認は後回しにできる。

---

## フェーズ別ステータス

### Phase 1: AI-Ready Pipeline（Markdown 初期スコープ）

| 領域 | 状態 | 根拠 |
|---|:---:|---|
| プロジェクト基盤 / Protocol Buffers | ✅ | `proto/record.proto` と生成済み `internal/pb/record.pb.go` |
| 状態管理 | ✅ | Firestore を既定、PostgreSQL を代替として選択可能（#63） |
| Google Drive 取得 | ✅ | サービスアカウント認証、Workspace ファイルの Export 対応 |
| ファイルパーサー / チャンク化 | ✅ | Markdown の解析・チャンク化（#25, #88） |
| Worker Pool / graceful shutdown | ✅ | 並行処理、キャンセル、テストを実装 |
| BigQuery ロード | ✅ | 冪等なチャンク更新、リトライ、Bronze テーブル |
| Worker エントリーポイント | ✅ | `cmd/worker` に統合済み（#6） |
| パイプライン結合テスト | ✅ | モックベースの E2E テスト（#46, #103） |
| Sentry 監視 | ✅ | Worker のテスト失敗通知が `#sentry-etl-alert` に届くことを 2026-09-30 に確認（#95, #114）。Slack 連携は 2026-10-18 に停止予定 |

### Phase 2: RAG Agent

| 領域 | 状態 |
|---|:---:|
| Vertex AI Embeddings | ❌ #36 |
| BigQuery Vector Search | ❌ #37 |
| RAG Agent CLI | ❌ #38 |
| RAG 評価 | ❌ #39 |

### Phase 3: BI & Delivery

| 領域 | 状態 | 根拠 |
|---|:---:|---|
| Silver / Gold レイヤー | ✅ | BigQuery View / Mart（#40） |
| Gold CSV の Drive 配信 | ✅ | `export-reports` への出力（#42） |
| データポータル | ✅ | `Ingestion health` ダッシュボード（#41, #112） |

---

## 運用・開発基盤

| 領域 | 状態 | 補足 |
|---|:---:|---|
| Terraform | ✅ | BigQuery / Firestore / データポータル IAM。worktree 間で state を安全に共有（#113） |
| CI / セキュリティ | ✅ | CI、OpenSSF Scorecard、Dependabot、依存脆弱性対応 |
| ユニットテスト | ✅ | `go test ./...` 全パッケージ成功（2026-10-11） |
| Protobuf 再生成 | 🚧 | 生成物はあるが、`Makefile` に `protoc` ターゲットは未追加 |

## 次の判断

Portfolio v1 は ETL / BI の範囲で完了とする。RAG は #23 を親とする Phase 2 の #36〜#39 で実装する将来拡張であり、Vertex AI の有料実行は本スコープの必須条件に含めない。

Cloud Run / Cloud Scheduler による定期実行も将来構成であり、現行スコープの必須要件ではない。
