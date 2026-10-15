# 振り返りの年月ラベル

先頭の空行を除いた最初の行が `# YYYYMM-N` 形式の場合、見出しから年月と番号を抽出する。ファイル名や本文中の別の見出しは代用しない。現時点では前置きや追加の文字を含むタイトルは未対応とする。

| 列 | BigQuery の型 | 値の例 | 意味 |
|---|---|---|---|
| `reflection_month` | nullable STRING | `2026-10` | タイトルに記載された年月 |
| `reflection_sequence` | nullable INTEGER | `1` | 月内の週または回数を表す番号 |
| `reflection_start_date` | nullable DATE | `NULL` | 正確な対象期間の開始日 |
| `reflection_end_date` | nullable DATE | `NULL` | 正確な対象期間の終了日 |

番号は第何週かを確定する情報ではない。`1` から月初の7日間などを推測せず、正確な対象期間は未設定のままにする。番号は1以上の int32 範囲とし、年月や番号が不正な見出しでは年月・番号とも未設定にする。本文と出典、取り込み日時は引き続き保存する。

年月ラベルは分割前の本文から一度だけ抽出し、全チャンクへ同じ値を付ける。Raw の `chunks` に保存し、`chunks_current` と Silver の `silver_chunks` に公開する。既存行には値がないため、追加列は NULLABLE とする。

## 月による取得

Silver のチャンクを読む例は次のとおり。`@month` には STRING 型で `2026-10` を渡す。

```sql
SELECT file_id, chunk_index, content, document_name, drive_url,
       reflection_month, reflection_sequence
FROM `<project>.etl_silver.silver_chunks`
WHERE reflection_month = @month
ORDER BY file_id, chunk_index
```

この条件はタイトルの年月による検索であり、実際の振り返り対象期間との重複検索ではない。正確な期間が不明でも年月が分かれば月検索には含める。年月も不明な文書はこの検索に含まれない。

外部の Python / FastAPI アプリ向けの文書単位の取得には、Gold View `reflection_documents` を使う。公開列・月検索・検証・利用手順は [振り返り文書 View](reflection-documents.md) を参照。

## 適用と再処理

スキーマと View の定義変更だけでは実環境へ反映されない。実際の取り込みより前に、リポジトリのルートから次を実行し、計画を確認して適用する。

```sh
./scripts/terraform plan
./scripts/terraform apply
```

Raw の `chunks` → `chunks_current` → Silver の `silver_chunks` の順に更新する必要がある。View の `depends_on` でこの順序を指定する。指定がないと、上流の更新完了前に下流の View が追加列を参照して失敗する場合がある。

既存ファイルへ年月と出典を付け直す場合は、次を実行する。

```sh
REPROCESS_ALL=true go run ./cmd/worker
```

`REPROCESS_ALL` が文字列の `true` の場合、Drive の一覧取得で見つかった各ファイルを、状態管理 DB へ登録した後で `pending` に戻す。対象は `DRIVE_FOLDER_ID` の設定に従う。特定の ID だけを選ぶ機能は用意していない。指定フォルダ内の対象ファイルを一括で再処理する運用とする。

未指定・空文字・`false` の場合は通常の変更判定に従う。Firestore / PostgreSQL とも、本文のチェックサムが同じでも文書名が変われば再処理対象にする。`REPROCESS_ALL=true` を `.env` に残すと毎回強制再処理するため、一度だけの再処理では上記の起動時指定を使う。

登録に失敗したファイルは強制的な状態更新を行わず、次のファイルへ進む。強制再処理のための状態更新に失敗した場合は、ファイル ID を含むエラーを返して Worker Pool の起動前に終了する。それまでに成功した状態更新は取り消さない。

Raw は追記方式なので再処理では保存行が増えるが、`chunks_current` は同じ本文バージョン・チャンク番号の最新取り込みを選択し、Silver へ公開する。`drive_files` は重複除去 View を持たず、再処理でメタデータ行が追加される。

## 検証結果

2026-10-15 にユーザーが Terraform の適用と `REPROCESS_ALL=true` による再処理を実施した。Worker は Drive の3文書を検出して正常終了した。Silver に対する `SELECT DISTINCT` で、3文書のタイトルに対応する年月・番号、文書名、Drive URL を取得できた。実文書の本文・ファイル名・ID・URLはここには記録しない。

ローカルの `go test ./...` は成功した。Firestore の一時エミュレータと一時 PostgreSQL では、改名時の再処理判定テストも成功した。強制再処理の未指定・false・true、状態更新失敗、登録失敗後の継続は、`cmd/worker/reprocess_test.go` のモックテストで検証した。CI の実行結果とは区別する。

## 保持期間と後続の作業

現在の BigQuery Sandbox のテーブル保持期間は最大60日であり、通常の View は元テーブルの期限切れを防がない。長期参照の保持方針と外部アプリの読み取り権限は #129 で利用手順に記録する。

正確な対象期間の抽出は未実装で、開始日・終了日は未設定のままとする。Drive 上で削除・移動されたファイルの検知は今回追加していない。既存の tombstone は、検出して取り込んだファイルの本文が空になったことを表し、Drive 上の削除を表すものではない。

Go の合成データによるテストと Terraform の静的検証は、実 BigQuery 上の SQL 検証とは区別する。実文書本文はテストデータへ保存しない。
