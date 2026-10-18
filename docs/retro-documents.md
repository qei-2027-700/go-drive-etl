# 外部アプリ向けの振り返り文書 View

`etl_gold.retro_documents` は、既存の `reflection_documents` を介して `etl_silver.silver_chunks` の文書を取得し、列名をretroに揃える通常の View である。本文を二重保存せず、参照時に SQL を実行する。実際の dataset 名は Terraform の変数に従う。

## 公開するデータ

| 列 | 型 | 意味 |
|---|---|---|
| `file_id` | STRING | 文書 ID。1文書につき1行 |
| `document_name` / `drive_url` | STRING | 最新の文書名・出典。既存データでは NULL を許容 |
| `content` | STRING | チャンクを番号順に結合した本文 |
| `content_version` | STRING | 元の本文の SHA-256。結合後の本文のハッシュではない |
| `ingested_at` | TIMESTAMP | 当該文書の最新チャンク取り込み日時 |
| `retro_month` | STRING | タイトル由来の年月。例: `2026-10`。不明なら NULL |
| `retro_sequence` | INTEGER | タイトル由来の番号。不明なら NULL |
| `retro_start_date` / `retro_end_date` | DATE | 規約に基づく対象期間。不明なら NULL |

最新版選択・チャンクの重複除去・空文書 tombstone の除外は、上流の `chunks_current` が担う。Gold は `file_id` ごとに `chunk_index` 昇順で本文を結合する。区切りは改行2文字（`\n\n`）とする。既存パーサーは空白を正規化し、長い節を固定長で分割するため、原文との完全一致は保証しない。長い節の途中にも区切りが入る。

文書名・出典・年月などは、取り込み日時が最新のチャンクから STRUCT としてまとめて選ぶ。同じ日時では `chunk_index` が小さい行を選ぶ。上流では同じ取り込みの全チャンクに同じメタデータを付ける。NULL を過去の値で補わない。

年月不明の文書を含め、Silver の全現行文書を公開する。文書種別の自動判定は行わない。Drive 上の削除・移動は検知していないため、それらの文書が結果から除外される保証はない。tombstone は取り込んだ本文が空になったことを表す。

## パラメータ付き月検索

Python / FastAPI 側から、`@month` に STRING 型の `2026-10` を渡す。SQL 文字列への値の埋め込みは行わない。

```sql
SELECT file_id, document_name, drive_url, content, content_version,
       ingested_at, retro_month, retro_sequence,
       retro_start_date, retro_end_date
FROM `<project>.etl_gold.retro_documents`
WHERE retro_month = @month
ORDER BY retro_sequence, file_id
```

この条件はタイトルの年月による分類であり、実際の対象期間の重複検索ではない。年月不明の文書は月検索から除外する。期間不明でも年月が分かれば月検索に含める。対象期間の計算と重複検索は [sprintの対象期間](retro-sprint-periods.md) を参照する。取り込み日や月初・月末で補わない。

## 権限・料金・保持期間

通常の View の参照には、クエリ実行プロジェクトの `roles/bigquery.jobUser` と、View および参照先の各 View・元テーブルに対する読み取り権限が必要である。#137 の実装では、専用アカウントに新旧のGold Viewと上流3リソースの計5リソースの読み取り権限を個別に付与する。Authorized View は使用しない。公開範囲、認証、適用後の検証は [アプリ用アカウントの検証手順](retro-app-access.md) を参照する。[Google Cloud の権限説明](https://docs.cloud.google.com/bigquery/docs/control-access-to-resources-iam)

既存の BI 用アカウントは Gold dataset の読み取り権限を持つが、Raw / Silver の権限は持たないため、この通常の View の本文を取得するには不足する。BI の権限を広げる変更や、この View を Authorized View にする変更では、本文へのアクセス範囲も確認する。既存 Mart の SQL、CSV 出力対象、BI 設定は変更しない。

通常の View は参照時に元データを読み取る。月検索や LIMIT によって読み取り量が必ず小さくなるとは限らない。課金を有効化した環境では dry run で見積もり、利用側の `maximum_bytes_billed` を設定する。Sandbox の無料枠にも上限がある。[クエリ費用の管理](https://docs.cloud.google.com/bigquery/docs/best-practices-costs)

Sandbox では元テーブルと View 自体に最大60日の期限がある。通常の View は元テーブルの期限切れを防がず、ETL の追記だけでテーブルの有効期限が延びることもない。長期参照には課金有効化と、既存リソースを含む有効期限の見直しが必要である。保持設定の変更は今回含めない。[Sandbox の制約](https://docs.cloud.google.com/bigquery/docs/sandbox)

## 検証と実環境への適用

静的検証はリポジトリのルートから実行する。

```sh
./scripts/terraform fmt -check
./scripts/terraform validate
```

合成データの検証スクリプトは Terraform 内の実際の SQL を読み、Raw・最新版選択・Silver・Gold を一時テーブルで再現する。SQL をテスト用に複製しない。

```sh
./scripts/test-retro-documents > /tmp/reflection-documents-test.sql
bq query --use_legacy_sql=false --maximum_bytes_billed=268435456 \
  < /tmp/reflection-documents-test.sql
```

クエリ実行プロジェクトは事前に確認する。すべて合成データと一時テーブルを使い、本番テーブルを読み書きしない。SQL の ASSERT で本文の順序、重複、旧版、メタデータ更新と NULL、空文書、長い Unicode チャンク、期間 NULL、パラメータ付き月検索を検証する。Terraform の検証と実 BigQuery 上の SQL 検証は区別して記録する。

実環境への適用は別途、`./scripts/terraform plan` の差分を確認してから `./scripts/terraform apply` を実行する。依存順は `chunks_current` → `silver_chunks` → `reflection_documents` → `retro_documents` とする。#128 の列と既存データが揃っていれば、Gold View の追加だけを理由に Worker を再実行する必要はない。

適用後はアプリ用アカウントで上記の月検索を実行し、文書 ID の一意性・本文・出典・NULL と読み取り権限を確認する。実文書本文や識別情報を検証ログ、Issue、ソース、テストデータへ保存しない。合成データの成功は、デプロイ済み View やアプリ用 IAM の検証完了を意味しない。

## 検証結果

2026-10-15 に `terraform fmt -check`・`terraform validate` と `git diff --check` が成功した。合成データの SQL を実 BigQuery で実行し、9件の ASSERT がすべて成功した。この合成データ検証では既存 Terraform の SQL を一時テーブルへ適用し、本番テーブルの読み書きは行っていない。

同日、ユーザーが `./scripts/terraform apply` を実行し、View の追加1件・変更0件・削除0件で成功した。続いてデプロイ済み View に `@month = '2026-10'` のパラメータ付き月検索を実行し、文書数・文書 ID のユニーク数・本文が空でない文書数・出典 URL が NULL でない文書数がすべて1件であることを確認した。本文や出典の内容自体の正しさ、他の月、アプリ用アカウントの読み取り権限は、この集計では未検証である。実文書本文・識別情報は記録しない。
