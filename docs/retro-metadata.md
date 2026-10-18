# 振り返りの年月ラベル

先頭の空行を除いた最初の行が `# YYYYMM-N` 形式の場合、見出しから年月と番号を抽出する。ファイル名や本文中の別の見出しは代用しない。現時点では前置きや追加の文字を含むタイトルは未対応とする。

| 列 | BigQuery の型 | 値の例 | 意味 |
|---|---|---|---|
| `retro_month` | nullable STRING | `2026-10` | タイトルに記載された年月 |
| `retro_sequence` | nullable INTEGER | `1` | 月内の週または回数を表す番号 |
| `retro_start_date` | nullable DATE | `2026-10-05` | 規約に基づく対象期間の開始日。不明なら NULL |
| `retro_end_date` | nullable DATE | `2026-10-11` | 規約に基づく対象期間の終了日。不明なら NULL |

従来の番号は第何週かを確定する情報ではない。新たに合意した規約の適用対象を除き、正確な対象期間は未設定のままにする。番号は1以上の int32 範囲とし、年月や番号が不正な見出しでは年月・番号とも未設定にする。本文と出典、取り込み日時は引き続き保存する。

年月ラベルは分割前の本文から一度だけ抽出し、全チャンクへ同じ値を付ける。Raw の `chunks` から `chunks_current`・Silver・Gold まで、4列を `retro_*` に統一する。型と NULLABLE は維持し、不明な値を過去の値や取り込み日時で補わない。

## 月による取得

Silver のチャンクを読む例は次のとおり。`@month` には STRING 型で `2026-10` を渡す。

```sql
SELECT file_id, chunk_index, content, document_name, drive_url,
       retro_month, retro_sequence
FROM `<project>.etl_silver.silver_chunks`
WHERE retro_month = @month
ORDER BY file_id, chunk_index
```

この条件はタイトルの年月による検索であり、実際の振り返り対象期間との重複検索ではない。正確な期間が不明でも年月が分かれば月検索には含める。年月も不明な文書はこの検索に含まれない。

外部の Python / FastAPI アプリ向けの文書単位の取得には、Gold View `retro_documents` を使う。公開列・月検索・検証・利用手順は [振り返り文書 View](retro-documents.md) を参照。

## 適用と再処理

既存環境の `reflection_*` からの移行は、[Raw列の移行手順](raw-retro-column-migration.md) に従う。Terraform のスキーマ変更だけでは列をリネームできない。Worker を止め、SQL で列をリネームしてから、refresh を有効にした新しい plan を確認して View を更新する。列リネームだけを理由とする Drive の再取り込みは不要である。

新規環境では新スキーマを作成してから Worker を起動する。Raw → `chunks_current` → Silver → Gold の順序は `depends_on` で指定する。

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

月曜日を基準にする規約と対象期間の計算・検索・再処理は [sprintの対象期間](retro-sprint-periods.md) を参照する。規約の適用開始月は2026年11月に確定した。実行環境で `RETRO_SPRINT_FROM_MONTH=2026-11` を設定すると有効になり、未設定では期間計算を無効にする。Drive 上で削除・移動されたファイルの検知は今回追加していない。既存の tombstone は、検出して取り込んだファイルの本文が空になったことを表し、Drive 上の削除を表すものではない。

Go の合成データによるテストと Terraform の静的検証は、実 BigQuery 上の SQL 検証とは区別する。実文書本文はテストデータへ保存しない。

## retroへの命名統一

| 対象 | 名前と移行後の扱い |
|---|---|
| Go・Protobuf | `RetroLabel`・`RetroPeriod`・`ExtractRetroLabel`、`retro_*` フィールド。Protobuf のフィールド番号・型・optional は維持する |
| Raw・`chunks_current`・Silver | `retro_month`・`retro_sequence`・`retro_start_date`・`retro_end_date`。旧列の別名は公開しない |
| Gold | `retro_documents` が Silver を直接集約する。旧 `reflection_documents` とその専用の読み取り権限は削除する |
| 設定とコマンド | `RETRO_SPRINT_FROM_MONTH`、`verify-retro-app`・`test-retro-documents`・`test-verify-retro-app` を使う。旧 reflection コマンドは削除する |

`RETRO_SPRINT_FROM_MONTH` が空欄・未指定なら期間計算を無効にする。2026年11月からの適用方針は変更しない。`REFLECTION_SPRINT_FROM_MONTH` は読み取らない。

サービスアカウントの実 ID `reflection-documents-app` は維持する。既存 state のサービスアカウント・IAM 等を引き継ぐ `moved` ブロックも残す。検証者入力の旧 `reflection_app_impersonator_members` と出力 `reflection_app_service_account_email` は、既存設定・権限を保つため互換用に残す。列や View の互換処理とは分けて扱う。

旧列・旧 Gold View の利用側は切替前に確認する。移行順序、削除保護、state、失敗時の復旧、plan の判定基準は [Raw列の移行手順](raw-retro-column-migration.md) を参照する。

### 2026-10-18の PR #143 時点の検証（#144 の適用前）

`go test ./...`、`go vet ./...`、`go build ./...`、検証スクリプトのオフラインテスト8件、Terraformの静的検証が成功した。Protobufの既存のフィールド番号・型・optional、適用開始月の有効・無効・不正値を確認した。Firestore・PostgreSQLの接続を必要とする既存テストは、実DBを用意しない通常のGoテストではスキップされるため、今回の名前変更で実DBを使う検証を行ったという意味ではない。

実BigQueryの一時テーブルで20件のASSERTが成功した。既存Raw列の新しい別名、Silverの新旧列、新旧Gold Viewの本文・期間・NULLが一致することを確認した。本番データは読み書きしていない。

既存の共有stateに対するTerraform planは追加2件・変更3件・削除0件だった。追加は新Gold Viewとその読み取り権限、変更は `chunks_current`・Silverの公開列追加とサービスアカウントの表示名である。既存リソースの削除・再作成はなかった。applyと、デプロイ済みの新Viewを専用アカウントで読む検証はまだ行っていない。
