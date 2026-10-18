# 振り返りの年月ラベル

先頭の空行を除いた最初の行が `# YYYYMM-N` 形式の場合、見出しから年月と番号を抽出する。ファイル名や本文中の別の見出しは代用しない。現時点では前置きや追加の文字を含むタイトルは未対応とする。

| 列 | BigQuery の型 | 値の例 | 意味 |
|---|---|---|---|
| `retro_month` | nullable STRING | `2026-10` | タイトルに記載された年月 |
| `retro_sequence` | nullable INTEGER | `1` | 月内の週または回数を表す番号 |
| `retro_start_date` | nullable DATE | `2026-10-05` | 規約に基づく対象期間の開始日。不明なら NULL |
| `retro_end_date` | nullable DATE | `2026-10-11` | 規約に基づく対象期間の終了日。不明なら NULL |

従来の番号は第何週かを確定する情報ではない。新たに合意した規約の適用対象を除き、正確な対象期間は未設定のままにする。番号は1以上の int32 範囲とし、年月や番号が不正な見出しでは年月・番号とも未設定にする。本文と出典、取り込み日時は引き続き保存する。

年月ラベルは分割前の本文から一度だけ抽出し、全チャンクへ同じ値を付ける。Raw の `chunks` は互換性のため従来の `reflection_*` 列へ保存し、`chunks_current` と Silver の `silver_chunks` で `retro_*` の別名も公開する。既存行には値がないため、追加列は NULLABLE とする。

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

月曜日を基準にする規約と対象期間の計算・検索・再処理は [sprintの対象期間](retro-sprint-periods.md) を参照する。規約の適用開始月は2026年11月に確定した。実行環境で `REFLECTION_SPRINT_FROM_MONTH=2026-11` を設定すると有効になり、未設定では期間計算を無効にする。Drive 上で削除・移動されたファイルの検知は今回追加していない。既存の tombstone は、検出して取り込んだファイルの本文が空になったことを表し、Drive 上の削除を表すものではない。

Go の合成データによるテストと Terraform の静的検証は、実 BigQuery 上の SQL 検証とは区別する。実文書本文はテストデータへ保存しない。

## retroへの命名変更と互換性

コードの型・関数・ファイル名と新しい公開名では、振り返りを `retro` と呼ぶ。既存データや利用側を壊さないため、外部に保存済みの名前は互換用に残す。

| 対象 | 新しい名前と既存利用側の扱い |
|---|---|
| Go・Protobuf | `RetroLabel`・`RetroPeriod`・`ExtractRetroLabel`、`retro_*` フィールド。Protobufの番号と既存のcamelCaseのProtoJSON名は維持する |
| Rawの保存列 | 既存の `reflection_*` を維持し、過去データも新しい取り込みも同じ列へ保存する |
| `chunks_current`・Silver | 旧 `reflection_*` に加え、同じ値を持つ `retro_*` 列を公開する |
| Gold | 新しい `retro_documents` と従来の `reflection_documents` を併存させる。本文・期間・最新版の選択は同じで、新Viewは列名だけを変える |
| 設定とコマンド | 新名は `RETRO_SPRINT_FROM_MONTH` と `verify-retro-app`・`test-retro-documents`・`test-verify-retro-app`。旧環境変数と旧コマンドも互換用に残す |

新環境変数が存在する場合は、空欄も含めて新名を優先する。新名が未設定のときだけ `REFLECTION_SPRINT_FROM_MONTH` を使う。旧設定を残した環境でも、新名を空欄にすれば期間計算を無効にできる。

サービスアカウントの実IDは `reflection-documents-app` のまま維持する。Terraformの識別子はretroへ変更し、`moved` ブロックで既存stateのリソースを引き継ぐ。検証者の入力変数は `retro_app_impersonator_members` を新名とし、旧 `reflection_app_impersonator_members` との和集合を使って既存の権限を失わないようにする。出力名も新旧を併存させる。

適用前に `./scripts/terraform plan` で、既存View・サービスアカウント・IAMの削除や再作成がないことを確認する。適用順は `chunks_current` → Silver → 旧Gold View → 新Gold Viewと読み取り権限とする。新Viewは通常のViewなので、専用アカウントには新Viewに加えて旧View・Silver・Rawの読み取り権限も必要になる。既存のアカウントIDや本文の公開範囲を変えるものではない。

Terraformを適用してから利用側のクエリを `retro_documents` / `retro_*` に切り替える。旧View・旧列は引き続き使えるため、利用側を同時に切り替える必要はない。この命名変更だけを理由とするデータ再処理は不要である。旧 `verify-reflection-app` は旧Viewを、新 `verify-retro-app` は新Viewを検証する。

### 2026-10-18の命名変更の検証

`go test ./...`、`go vet ./...`、`go build ./...`、検証スクリプトのオフラインテスト8件、Terraformの静的検証が成功した。Protobufの既存のフィールド番号・ProtoJSON名、旧環境変数、新設定の優先順位を合成データで確認した。Firestore・PostgreSQLの接続を必要とする既存テストは、実DBを用意しない通常のGoテストではスキップされるため、今回の名前変更で実DBを使う検証を行ったという意味ではない。

実BigQueryの一時テーブルで20件のASSERTが成功した。既存Raw列の新しい別名、Silverの新旧列、新旧Gold Viewの本文・期間・NULLが一致することを確認した。本番データは読み書きしていない。

既存の共有stateに対するTerraform planは追加2件・変更3件・削除0件だった。追加は新Gold Viewとその読み取り権限、変更は `chunks_current`・Silverの公開列追加とサービスアカウントの表示名である。既存リソースの削除・再作成はなかった。applyと、デプロイ済みの新Viewを専用アカウントで読む検証はまだ行っていない。
