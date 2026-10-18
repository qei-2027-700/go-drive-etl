# Raw の reflection 列を retro に移行する（#144）

## 方針と対象

既存の Raw `chunks` を削除せず、4列を1つの `ALTER TABLE` 文でリネームする。本文・取り込み情報・型・NULL・行数・保持期間は変更しない。Drive の再取り込みや、Firestore / PostgreSQL の取り込み状態の初期化は不要である。

| 旧列 | 新列 | 型・モード |
|---|---|---|
| `reflection_month` | `retro_month` | STRING / NULLABLE |
| `reflection_sequence` | `retro_sequence` | INTEGER / NULLABLE |
| `reflection_start_date` | `retro_start_date` | DATE / NULLABLE |
| `reflection_end_date` | `retro_end_date` | DATE / NULLABLE |

移行後の経路は `chunks` → `chunks_current` → `silver_chunks` → `retro_documents` となる。旧列の別名、旧 Gold `reflection_documents`、そのテーブル専用のアプリ読み取り権限、旧 reflection コマンドは削除する。通常の View を使い、アプリの SA 実 ID、BI・ETL の IAM、検証者の既存設定、保持期限は維持する。

Google provider は `~> 6.0`、lock は **6.50.0** である。この版の [schema 差分の実装](https://github.com/hashicorp/terraform-provider-google/blob/v6.50.0/google/services/bigquery/resource_bigquery_table.go) は、列削除と列追加の同時変更を再作成と判定する。Terraform の定義変更を列リネームとして適用してはいけない。先に SQL を実行し、その後の refresh で実スキーマを読み込んでから plan を作り直す。`-refresh=false`、`ignore_changes`、state の手編集で差分を隠さない。

[BigQuery の列リネーム仕様](https://docs.cloud.google.com/bigquery/docs/reference/standard-sql/data-definition-language#alter_table_rename_column_statement) に従い、事前に row access policy・制約・partition / clustering の対象外であることを確認する。列に付いた Catalog のタグ等がある場合は消失するため、移行前に確認する。このリポジトリではこれらを定義していない。参照 View の SQL は自動更新されない。

## 1. 利用側と適用状態を確認する

PR #143 のマージを前提とする。共有 state と実テーブル・View の適用状態も別に確認する。PR #143 が未適用なら、その版（`fc42119`）で plan を確認して先に適用する。Git のマージだけを実環境への適用完了とは扱わない。

リポジトリ内の利用側は `internal/etl/worker_pool.go`、Terraform の3段の View、`verify-retro-app` に統一した。Gold Mart の `sql/refresh_gold_marts.sql` は対象4列を参照しない。外部の FastAPI、手動クエリ、scheduled query、BI のカスタム SQL 等はリポジトリから確認できないため、所有者が旧列・旧 View の参照を確認する。必要なら対象 dataset の `INFORMATION_SCHEMA.VIEWS` とリージョンの `INFORMATION_SCHEMA.JOBS_BY_PROJECT` を参照し、結果は件数・有無だけ記録する。履歴がないことだけで未使用と断定しない。外部利用側の切替準備ができるまで旧 View は削除しない。

全 worktree の Terraform 操作を止め、正本の共有 state と tfvars を確認する。state と tfvars のバックアップは所有者だけが読める共有ディレクトリに置き、Git・Issue・検証ログには載せない。`scripts/terraform apply` は適用前に共有 state を自動バックアップする。移行前のコミット、Worker の起動方法、View の定義と各 IAM の対象を控える。state をバックアップへ戻すだけでは実リソースを復元できない。

## 2. 旧 Gold View の削除保護だけを先に解除する

この変更後の構成は旧 View のリソースを持たないため、先に PR #143 の構成を使う一時作業ディレクトリで保護を解除する。既存の `moved` により、旧 state の `google_bigquery_table.reflection_documents` は `google_bigquery_table.retro_documents_legacy` へ引き継ぐ。テーブル ID は引き続き `reflection_documents` である。

`iac/silver_gold.tf` の **`retro_documents_legacy` だけ** `deletion_protection = false` に変更し、その構成で通常の plan を作る。期待する変更はこの保護解除1件のみで、Raw のスキーマ変更やリソースの追加・削除・再作成がないことを確認して適用する。Raw `chunks` と残す View の保護は `true` のままとする。`terraform state rm` や手動の View 削除は使わない。

この段階では旧 Worker と利用側は動作を続けられる。以後は #144 の構成へ戻し、一時作業ディレクトリからの Terraform 操作を終了する。

## 3. Worker と利用側を止め、移行前 plan を確認する

手動・ローカル・定期実行を含むすべての Worker 起動元を停止し、実行中の Worker と BigQuery Load Job の完了を待つ。途中の処理を強制終了した場合は、取り込み状態を確認して復旧してから進む。アプリや BI の対象 View を参照するクエリも保守時間中は止める。列リネームから全 View の更新が終わるまでは参照できない時間がある。

移行後の Worker を用意するが、まだ起動しない。Worker が再起動しないことを確認してから、#144 の構成で次を実行する。

```sh
./scripts/terraform plan -input=false
```

この段階の plan は確認専用で、**適用しない**。旧実スキーマに対して4列の削除・追加として表示され、Raw `chunks` の再作成が計画され得る。対象を記録して、SQL によるリネーム後に必ず作り直す。削除保護を外してこの再作成を通してはいけない。

## 4. 列をリネームし、新しい plan で View を切り替える

`PROJECT_ID` と `RAW_DATASET` は正本の設定に合わせて指定する。生成コマンドは SQL を出力するだけで、認証や実環境への接続を行わない。

```sh
./scripts/retro-column-migration --project "$PROJECT_ID" --dataset "$RAW_DATASET" \
  > /tmp/retro-columns-forward.sql
```

実行先・既存スキーマが表の旧4列と一致し、新4列が存在しないことを `bq show --schema` 等で確認する。型とモードも確認する。生成 SQL は次の1文であり、`IF EXISTS` は使わない。部分移行や異なるスキーマを黙って通さないためである。

```sql
ALTER TABLE `<project>.<raw_dataset>.chunks`
  RENAME COLUMN reflection_month TO retro_month,
  RENAME COLUMN reflection_sequence TO retro_sequence,
  RENAME COLUMN reflection_start_date TO retro_start_date,
  RENAME COLUMN reflection_end_date TO retro_end_date;
```

実環境への変更を実施する段階で、この SQL を管理者の認証で実行する。アプリ用アカウントへスキーマ更新権限を付けない。

```sh
bq --project_id="$PROJECT_ID" --location="$BQ_LOCATION" query \
  --use_legacy_sql=false < /tmp/retro-columns-forward.sql
./scripts/terraform plan -input=false -out=/tmp/retro-columns-after.tfplan
```

新しい plan で次を確認する。保存 plan は比較・確認用とする。リポジトリの wrapper は apply 時に共有 tfvars を渡すため、保存 plan を直接渡さず、後述の通常の apply で再計画した差分が一致することを確認して承認する。

1. Raw `chunks` の列削除・追加・再作成がなく、4列の型と NULLABLE が定義と一致する。保持期限にも変更がない。
2. `chunks_current`・`silver_chunks`・`retro_documents` は SQL の更新だけで、Raw → Silver → Gold の依存順になっている。
3. 削除は旧 `reflection_documents` と、その専用アプリ IAM のみ。SA、残す4リソースへのアプリ IAM、BI・ETL の IAM に意図しない変更がない。
4. 予想外の `create`・`delete`・`-/+` があれば停止する。移行前に保存した plan は再使用しない。

```sh
./scripts/terraform apply
```

通常の apply により旧 View とその IAM を削除する。削除順は Terraform の依存グラフに従う。保守時間中は利用側を停止しているため、旧 View の削除と新 View の更新の途中状態を公開しない。削除保護エラーが出たら手動削除せず、手順2の適用状態を確認する。

## 5. 検証してから再開する

Raw の実スキーマに新4列だけが存在することを確認する。`chunks_current`・Silver の旧別名がなく、Gold が Silver を直接参照し、旧 View がなくなったことも確認する。`INFORMATION_SCHEMA` の反映が遅れている場合はテーブル API で確認する。

アプリ用 SA で [月検索・権限の検証](retro-app-access.md) を実行する。期間検索は [期間の重複条件](retro-sprint-periods.md) に従い、`@query_start`・`@query_end` を DATE パラメータとして渡し、`COUNT(*)` のみを返す。既存期間がすべて NULL なら0件でも成功し得るので、合成データの結果と区別する。年月不明、期間不明の NULL 件数も確認する。実文書本文・ID・文書名・出典 URL・認証情報はログ等へ出力しない。

最終 plan が `No changes` となり、月・期間検索と4リソースの読み取り、BI・ETL の既存権限が維持されていることを確認してから、新 Worker と利用側を再開する。通常の変更検知で取り込みを続け、`REPROCESS_ALL` はこの列名変更のためには指定しない。

## 失敗時の復旧

Worker と利用側を停止したまま、実スキーマ・View・state の状態を確認する。DDL の通信タイムアウトは未実行と断定せず、4列の実際の名前を確認する。部分状態や旧新混在を見つけた場合は、その状態に合わせて復旧を決め、生成 SQL を再実行しない。

リネーム済みで元へ戻す場合は、次で逆向きの SQL を生成する。

```sh
./scripts/retro-column-migration --project "$PROJECT_ID" --dataset "$RAW_DATASET" \
  --rollback > /tmp/retro-columns-rollback.sql
bq --project_id="$PROJECT_ID" --location="$BQ_LOCATION" query \
  --use_legacy_sql=false < /tmp/retro-columns-rollback.sql
```

その後、移行前（PR #143）の構成で refresh を有効にした plan を作り、Raw の再作成がないことを確認して適用する。Raw → `chunks_current` → Silver → 旧 Gold → 新 Gold の SQL と旧 View の IAM を復元する。削除済みの旧 View は元の SQL で再作成される。旧 View の削除保護も `true` に戻す。`retro_documents` のリソースアドレスは前後で同じであり、旧 View を新 View へ `state mv` してはいけない。

Raw の逆リネームと View の復元、旧 Worker の配備、月・期間・権限の検証が終わってから処理を再開する。列名のみを戻すため、本文・取り込み日時・取り込み状態の復元や Drive の再取り込みは不要である。Terraform の途中失敗では成功済みの変更も残るので、バックアップ state の上書きではなく、現状の state と新しい plan で整合させる。

列リネームが使えず再作成が必要になった場合は、この手順を中断する。Raw の削除保護を解除せず、データのバックアップ、取り込み状態、Drive の再取得可能性と再処理範囲を確認し、実際の plan に基づく別の手順とする。

## 検証の区分

Go テスト、Terraform の静的検証、実 BigQuery の合成データ検証、実環境の plan、実環境への適用結果を分けて記録する。`scripts/test-retro-documents` は合成データと一時テーブルだけを使い、移行前後の Raw・各 View の値と重複行数、最新版選択、NULL、空文書 tombstone、月・期間検索、逆リネームを検証する。本番のテーブル・View は読み書きしない。

### 2026-10-18 の #144 実装時の検証

| 区分 | 結果 |
|---|---|
| Go | `go test ./...` が成功。旧列が出力されない検証を加えた後に `go test ./internal/etl` も成功。`go vet ./...`・`go build ./...` が成功。外部 DB を起動する統合検証は実施していない |
| オフライン | `python3 scripts/test-verify-retro-app` の7件が成功 |
| Terraform 静的検証 | `./scripts/terraform fmt -check`・`./scripts/terraform validate` が成功 |
| 実 BigQuery・合成データのみ | 一時テーブルに対する21件の ASSERT が成功。最初は累計256 MiBの上限に達したため、合成テストだけ上限を1 GiBにして再実行した。アプリ検証の既定256 MiBは変更していない |
| 実環境の移行前 plan | Raw `chunks` の再作成1件、`chunks_current`・Silver・新 Gold の SQL 更新3件、旧 Gold View と対応するアプリ IAM の削除各1件。SA・BI・ETL の IAM と保持期間に変更なし。この plan は適用していない |
| 実環境への適用 | 列リネーム、削除保護解除、View / IAM の更新・削除、適用後の月・期間検索は未実施。リネーム後の実環境 plan も未確認 |

ローカルの詳細ログは Git 管理外の `/tmp/retro-144-checks/` に保存した。Go・Terraform・合成 SQL・文書の検査結果を分けて保存し、本文や文書識別情報を含む本番データは取得していない。外部利用側の参照確認と、上記の段階適用は実施時に必要である。

### 2026-10-18 の #146 本番移行

ユーザーが外部アプリを利用していないことと、保守中に Worker・Terraform を起動しないことを確認し、このセッションでの移行を了承した。Looker Studio の既存 `Ingestion health` を実画面で確認したところ、データソースは `etl_gold.mart_ingestion_daily` と `etl_gold.mart_file_latest` の2つで、どちらもテーブル接続・オーナー認証だった。両方とも今回の変更対象ではなく、レポートの設定は変更していない。scheduled query の transfer config は0件、直近7日間の Load Job は9件すべて完了しており、ローカルの Worker・Terraform も実行中ではなかった。

適用先は `go-drive-etl-507314`、リージョンは `asia-northeast1`。実行者は `qlpt2020@gmail.com`、共有 state と tfvars は既定の正本を使用した。バックアップと保存 plan は所有者だけが読める `~/.local/state/go-drive-etl/retro-146-private/` に保管した。保守中の失敗時は利用側を停止したまま、逆リネームと `fc42119` の View / IAM 構成による復旧を行う手順とした。state の上書きや Raw の再作成は行っていない。

| 段階 | 確認・適用結果 |
|---|---|
| 旧 View の保護解除 | `fc42119` の一時構成で `retro_documents_legacy.deletion_protection` の `true` → `false` だけを変更。plan・apply とも追加0件・変更1件・削除0件。Raw と残す View の保護は維持 |
| リネーム前 | Raw は30行・24,447 bytes。旧4列の型・NULLABLE を確認。row access policy は0件で、API のスキーマ・メタデータに partition / clustering・制約・policy tag・resource tag はなかった。移行前 plan の Raw 再作成は適用していない |
| Raw の DDL | `scripts/retro-column-migration` が生成した1文の `ALTER TABLE` が成功。旧4列が新4列へ変わり、その他のスキーマ・型・モード・行数・bytes・保持期限は移行前と一致。列名を揃えて全列から計算した集計フィンガープリントも一致し、本文・取り込み情報を含む値の維持を確認 |
| リネーム後の plan | refresh を有効にした新しい plan で Raw は変更なし。残す3 View は SQL だけの更新で、削除は旧 View とその専用アプリ IAM だけ。apply も追加0件・変更3件・削除2件で成功 |
| View・既存 IAM | `chunks_current` → `silver_chunks` → `retro_documents` に旧列名・旧別名はなく、Gold は Silver を直接参照。旧 `reflection_documents` は API で404。残す IAM・SA の state 属性と共有 tfvars は移行前と一致 |

検証者への Token Creator は `-var` でアプリ SA だけに一時付与し、共有 tfvars は変更しなかった。追加権限の反映後、`reflection-documents-app@go-drive-etl-507314.iam.gserviceaccount.com` への impersonation で `scripts/verify-retro-app` が成功した。別クエリの `SESSION_USER()` でも同じ SA を確認した。クエリごとの費用上限は256 MiBである。

| アプリ SA による検証 | 結果 |
|---|---|
| `@month = '2026-10'` の月検索 | 文書数・一意数・空でない本文数・出典ありは各1件。文書名 NULL は0件、開始日・終了日 NULL は各1件 |
| View 全体の NULL | 全3文書の年月 NULL は0件、開始日・終了日 NULL は各3件 |
| DATE パラメータの期間検索 | `2026-10-01`〜`2026-10-31` の両端を含む重複条件で実行成功、0件。全期間が NULL のため、実データによる期間抽出の成功とは扱わない |
| 4リソースの実読み取り | Raw `chunks` は30行、`chunks_current` は15行、Silver は15行、Gold は3文書。返したのは件数だけ |

一時検証権限の削除は追加0件・変更0件・削除1件の plan と apply で成功し、SA の実 IAM ポリシーからも検証者の binding がなくなった。削除後の `plan -detailed-exitcode` は終了コード0で `No changes` だった。発行済み短期トークンは binding 削除だけでは失効せず、有効期限まで利用できる可能性がある。

Policy Troubleshooter による実効権限38件は、すべて想定どおりだった。REST API の `GRANTED` / `NOT_GRANTED` を、それぞれ `CAN_ACCESS` / `CANNOT_ACCESS` に対応させて確認した。

| 権限の対象 | 判定 |
|---|---|
| アプリ → 残す4リソースの `getData` | 4件すべて許可 |
| アプリ → 同4リソースの `updateData` / `update` / `delete` / `setIamPolicy` | 16件すべて不許可 |
| アプリ → 3 dataset の `tables.create` / `datasets.update` / `datasets.delete` | 9件すべて不許可 |
| アプリ → `drive_files`、BI → Raw 2リソース・Silver の `getData` | 4件すべて不許可 |
| BI → 新 Gold View・既存2 Mart の `getData`、アプリ・BI → `jobs.create` | 5件すべて許可 |

最初の権限確認では quota project の指定不足を修正し、その後の回数制限には間隔を空けた再試行で対応した。API エラーを拒否成功として数えていない。実データへの更新・削除による権限試験は行っていない。

最終 plan と権限確認の後、移行後コミットからビルドした Worker を既存の実行設定で通常実行した。`REPROCESS_ALL=false` を明示し、3ファイルの検出後にパイプラインが終了コード0で完了した。実行ログは本文・識別情報を保存せず、件数・終了コード・完了の有無だけを記録した。通常実行後も Raw は30行だった。Looker Studio のデータ更新後、18:08 の表示でファイル数3件・未埋め込みチャンク15件を確認した。Worker は手動の単発実行であり、常駐プロセスや schedule の再起動はない。

`go build ./...` と Worker のビルドは成功した。変更は本番移行の実績追記だけで、#144 で成功済みの Go テスト・合成データ21件の ASSERT は繰り返していない。実文書の本文・出典の内容の正しさ、NULL でない実期間の抽出、外部アプリの実行基盤は未検証である。

詳細ログは Git 管理外の `/tmp/retro-146-checks/` に保管した。本文・文書 ID・文書名・出典 URL・state / tfvars の内容・トークン・鍵は本記録と検証ログに保存していない。共有 tfvars の既存の未宣言変数 `scheduled_query_service_account_email` の警告は残っている。
