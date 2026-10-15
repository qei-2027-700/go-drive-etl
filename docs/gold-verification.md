# Gold・Drive CSVの再現可能な検証

`verify-gold` スキルと `cmd/verify_gold` は、現時点のSilverを期待値としてGoldとCSVを検証する。固定の文書数・チャンク数には依存しない。

## 設定と認証

リポジトリルートで `.env.example` を参考に `.env` を用意する。コマンドはgodotenvで値を読み、既存の環境変数を優先する。shellコードとしては実行しない。

| 設定 | 用途・既定値 |
| --- | --- |
| `BIGQUERY_PROJECT_ID` | 必須。既存Workerと同じプロジェクト |
| `BIGQUERY_GOLD_DATASET_ID` | `etl_gold` |
| `BQ_SILVER_DATASET_ID` | `etl_silver` |
| `BQ_LOCATION` | `asia-northeast1` |
| `DRIVE_EXPORT_FOLDER_ID` | 必須。既存CSV出力先 |
| `GOLD_DASHBOARD_URL` | レポートURL。未設定なら [画面仕様](looker-studio.md) の既存URLを使う |

BigQueryはADCを使う。既存の `GOOGLE_APPLICATION_CREDENTIALS` または `gcloud auth application-default login` の認証を用意する。Driveは既存Workerと同じくOAuthの3設定が揃えばユーザー認証、それ以外はADCを使う。読取にはBigQueryジョブ作成・Silver/Gold読取とDrive読取が必要。書込モードにはGoldテーブル更新とDriveファイル作成・更新も必要で、`bq` CLIの認証も必要になる。

## 実行

`cmd/verify_gold` のモックテストでは、通常実行で再集計・再出力を呼ばないこと、書込モードで2回出力すること、各段階の失敗後に後続処理を呼ばないこと、結果が重複せず途中の検証値を保持することを確認する。

```bash
# 合成データ・モックだけで検証。クラウドアクセスなし。
go test ./internal/goldverify ./internal/goldreport ./cmd/verify_gold ./cmd/worker

# 現状のGold/CSVを読む。再出力とID維持は未実施になる。
go run ./cmd/verify_gold

# Gold再集計とCSV出力2回を含む実環境検証。
go run ./cmd/verify_gold --refresh-and-export
```

書込モードは `scripts/refresh_gold_marts.sh` を呼ぶ。`BQ_PROJECT_ID` と `BQ_GOLD_DATASET_ID` はWorkerの設定から子プロセスへ渡すため、別の出力先と混同しない。CSVはWorkerと検証コマンドが共有する `internal/goldreport.Export` を使う。ETLの再実行はしない。通常のCIやスキル読込から書込を実行しない。

検証中はETL・Scheduled Query・他のCSV出力を止め、Silver/Goldを安定させる。同時更新で差分が出た場合は更新が終わってから再実行する。Gold更新後にCSV出力が失敗しても自動ロールバックはしない。

空のSilverは既定で失敗する。空であることを意図している場合だけ `--allow-empty` を追加する。Goldも0行、CSVもヘッダーのみで一致する必要がある。Silverが空でないのにGold/CSVが空なら、このフラグがあっても失敗する。

## 結果の判定

コマンドはUTCの開始・終了日時と段階別結果をJSONで出力する。各段階は成功・失敗にかかわらず1件だけ記録し、失敗時も途中の件数やCSV検証値を保持する。失敗後の段階は実行せず、未実施として残す。Goldの両方向の全列差分件数、期待・実件数、画面スコアカードの期待値、CSVのデータ行数・列数、重複件数、完全一致、2回目のID維持を記録する。`stable_id: false` は初回では比較未実施、2回目では失敗を示す。行数の比較は `EXCEPT DISTINCT` で消える重複行も検出する。

| 状態 | 対応 |
| --- | --- |
| 認証切れ・権限不足 | 失敗段階を確認し、認証を更新してから再実行する |
| 古いGold・欠落・余分な行・重複 | Gold比較が失敗。Silverが安定しているか確認し、必要なら書込モードで再集計する |
| CSV不一致・重複・ID変更 | 失敗。出力先、並行実行、既存重複を確認する。自動削除はしない |
| 画面のログイン要求 | 一部未確認。認証を迂回しない。人が開くURLと期待値を示す |
| APIとSQLが成功 | データ側の成功。画面描画は別に確認する |

直接ビルドしたバイナリの終了コードは失敗1、一部未確認2。`go run` では子プロセスの非ゼロ終了が1に包まれるため、JSONの `status` で区別する。画面はコマンドから判定しないため、データ検証が通ってもコマンド単体は `partial` になる。

ログイン済みブラウザで全期間を選び、`dashboard_files` と `dashboard_pending` をスコアカードと比較する。日次チャート・最新文書表も画面仕様に従って確認し、接続エラーがないことと実行日時を記録する。期間指定がある場合は同じ期間の値で比較する。書込モードが成功し、画面確認もできた場合に、担当者が全体成功を記録する。画面が見られなければ、データ側の結果を保ったまま理由とURLを残す。

## ログの保管と削除

成果物はOSの一時ディレクトリの `gold-verification-*` に保存する。ディレクトリは0700、`summary.json` と失敗時の `error.txt` は0600。リポジトリ外なのでGit管理しない。CSV本体とDriveファイルIDはメモリ内だけで比較し、保存しない。`error.txt` はAPIが返した識別子やSQLを含み得るため公開しない。必要な結果を確認したら、表示されたディレクトリを `rm -r /実際のパス/gold-verification-xxxx` で削除する。共有前にはsummaryも含め、レポートURLや集計値を公開してよいか確認する。

## 実環境での再実行結果（2026-10-15）

2026-10-15 11:30:08–11:30:18 UTCに `--refresh-and-export` で実行した。

| 段階 | 結果 |
| --- | --- |
| Gold再集計 | 成功 |
| Silver対Gold | 両Martの双方向差分0件。日次1行、文書別3行、Silver 15チャンク |
| CSV初回出力 | Drive OAuthの `invalid_grant` で失敗。以後のダウンロード・再出力・ID維持は未実施 |
| 画面 | 11:30:21 UTC更新の画面をログイン済みChromeで確認。文書数3・未埋め込み15が期待値と一致。2026-10-15のチャンク数15の点と、最新文書表3行を描画。接続エラー表示なし |
| 全体 | CSV検証が失敗したため未完了。Drive OAuth認証を更新して再実行する |

画面は [既存レポート](https://datastudio.google.com/u/1/reporting/4ea2ca0b-3c98-4184-93d8-e779652451e2/page/AF69F) を開いて確認した。CSV失敗時のsummaryと詳細エラーは、実行端末のOS一時ディレクトリ `gold-verification-228685222` に保存した。実文書IDやCSV本体はここへ転記していない。

### OAuth再認証後の再検証

2026-10-15 11:35:35–11:36:02 UTCに、OAuth再認証後の `--refresh-and-export` を実行した。

| 段階 | 結果 |
| --- | --- |
| Gold再集計・比較 | 成功。両Martの双方向差分0件 |
| 日次CSV | データ1行・6列。2回ともBigQuery出力と完全一致、重複0件 |
| 文書別CSV | データ3行・7列。2回ともBigQuery出力と完全一致、重複0件 |
| 再出力 | 両CSVともファイルID維持 |
| 画面 | ログイン済みChromeで再確認。文書数3・未埋め込み15、日次グラフ、最新文書表3行を表示。接続エラー表示なし |
| 全体 | データ検証と画面確認が成功。前回の認証エラーは解消 |

コマンドのsummaryは画面を自動判定しないため `partial`・`dashboard: unverified` のままになる。上の画面確認を合わせて全体成功と判定した。再実行ログは実行端末のOS一時ディレクトリ `gold-verification-614132591/summary.json` に保存した。
