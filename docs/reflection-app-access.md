# 振り返りアプリ専用アカウントのアクセス検証（#137）

## 設計と公開範囲

Terraform の `iac/reflection_app.tf` で `reflection-documents-app@<project_id>.iam.gserviceaccount.com` を新規作成する。ETL と `looker-studio-bi` から分離し、クエリ実行プロジェクトは `var.project_id` と同じにする。既存の Gold View は通常の View のまま使う。

| 対象 | 付与する権限 |
|---|---|
| `var.project_id` | `roles/bigquery.jobUser`（ジョブ実行） |
| Raw `chunks`、`chunks_current` | 各テーブル・View の `roles/bigquery.dataViewer` |
| Silver `silver_chunks` | View の `roles/bigquery.dataViewer` |
| Gold `reflection_documents` | View の `roles/bigquery.dataViewer` |

アプリは全現行文書の本文・文書名・出典・年月・期間に加え、Raw `chunks` に残る旧版・tombstone・取り込み情報も直接参照できる。月検索は公開範囲を限定するアクセス制御ではない。この範囲をアプリに公開してよいことを、適用前に確認する。`drive_files`、他の Mart、dataset 全体への権限は追加しない。

Gold 全体の読み取り権限を持つ BI への本文公開を避けるため、Authorized View は採用しない。既存の BI・ETL の IAM、SQL、CSV 配信は変更しない。上流の直接参照を禁止する必要が生じた場合は、BI が読めない別 dataset にアプリ向け Authorized View を分離する設計を別途行う。

`dataViewer` は読み取り・エクスポートを許可するが、元データの更新・削除は許可しない。プロジェクト・フォルダ・組織からの継承権限は追加の許可になり得るため、適用後に実効権限を確認する。[BigQuery のリソース単位の権限](https://docs.cloud.google.com/bigquery/docs/control-access-to-resources-iam)

## 認証

1. ローカル検証は `gcloud auth print-access-token --impersonate-service-account=...` で短期トークンを取得する。検証者を共有 tfvars の `reflection_app_impersonator_members` に明示する。初期値は空であり、そのままでは検証者への権限を追加しない。Token Creator はこのアカウントだけに付ける。IAM Service Account Credentials API と Policy Troubleshooter API は `google_project_service.reflection_app_access` で有効化する。定義削除時の API 無効化は行わない。
2. Google Cloud 上の Python / FastAPI は、このアカウントを実行リソースへアタッチし、ADC で認証する。クライアントの `project` は同じクエリ実行プロジェクトを明示する。実行基盤の作成・アタッチ権限は本変更に含めない。
3. Google Cloud 外で稼働する場合は Workload Identity Federation と当該アカウントの impersonation を使う。実行場所・外部 IdP は未確定のため、その信頼設定と実行時認証は未検証である。ローカル impersonation の成功を、実行基盤の認証成功とは扱わない。

サービスアカウント鍵は発行しない。検証スクリプトはユーザー認証へフォールバックせず、短期トークンをメモリ内だけで扱う。[BigQuery の認証方法](https://docs.cloud.google.com/bigquery/docs/authentication)

## 適用と検証

1. 共有 tfvars の project・dataset・location と検証者を確認し、次を実行する。plan の追加範囲と上記の公開範囲を確認してから apply する。今回の適用結果は末尾に記録する。

   ```sh
   ./scripts/terraform fmt -check
   ./scripts/terraform validate
   ./scripts/terraform plan -input=false
   # 差分とアクセス範囲の確認後に実行
   ./scripts/terraform apply
   ```

2. 適用後、データがある月を指定して実行する。非標準の Gold dataset / location なら引数で指定する。

   ```sh
   ./scripts/verify-reflection-app \
     --project '<project_id>' \
     --service-account 'reflection-documents-app@<project_id>.iam.gserviceaccount.com' \
     --month '2026-10'
   ```

   スクリプトは明示したアカウントで BigQuery REST API の dry run と本クエリを実行する。`reflection_month = @month` は STRING パラメータで渡す。文書単位の値を返さず、件数のみを取得する。選択月の文書数が1以上で、一意数・空でない本文数・空でない出典 URL 数が一致すれば終了コード0となる。本文・出典の内容自体の正しさは判定しない。既存データで出典が NULL の場合も、不足を見落とさないよう終了コード1にする。

   月内の文書名 NULL 数・期間開始/終了 NULL 数と、View 全体の年月 NULL 数も出力する。年月不明の文書は月検索から除外され、期間不明でも年月が一致すれば含まれる。NULL 数は観測結果として扱い、将来の期間メタデータ追加を失敗としない。NULL の選択ロジック自体は `scripts/test-reflection-documents` の合成データで検証する。

3. 管理者の認証で Policy Troubleshooter を使い、対象アプリの実効権限を確認する。実データへ DML・削除を実行しない。例は Raw `chunks` の更新権限である。

   ```sh
   gcloud policy-intelligence troubleshoot-policy iam \
     '//bigquery.googleapis.com/projects/<project_id>/datasets/<raw_dataset>/tables/chunks' \
     --principal-email='reflection-documents-app@<project_id>.iam.gserviceaccount.com' \
     --permission='bigquery.tables.updateData'
   ```

   4リソースそれぞれで `bigquery.tables.getData` が許可され、`bigquery.tables.updateData`・`bigquery.tables.update`・`bigquery.tables.delete`・`bigquery.tables.setIamPolicy` が許可されないことを確認する。3 dataset で `bigquery.tables.create`・`bigquery.datasets.update`・`bigquery.datasets.delete` も確認する。`drive_files` と既存 BI アカウントから Raw / Silver の `getData` は許可されないことを確認する。既存 BI の Gold 読み取りとジョブ実行の許可が維持されていることも確認する。結果が不明・条件付き・API無効・確認者の権限不足なら拒否成功とせず未検証と記録する。検証のためにアプリへ管理権限を付けない。[Policy Troubleshooter](https://docs.cloud.google.com/policy-intelligence/docs/troubleshoot-access)

4. 検証日、検証を開始した主体、impersonation先、プロジェクト、月、費用上限、集計値、各権限の判定、未検証事項を記録する。本文・文書 ID・文書名・出典 URL・トークン・鍵は Issue / ソース / ログに保存しない。API の生レスポンスやデバッグ出力も保存しない。スクリプトはエラーの詳細を出力しない。検証後、継続利用しない検証者は tfvars から外し、plan を確認して権限を削除する。

費用上限はクエリ1回につき既定256 MiB（`--maximum-bytes-billed`）である。全体の年月 NULL 集計を含むため、月検索だけより多く読み取る可能性がある。dry run 成功だけでは本クエリの成功を意味しない。アプリでも `maximum_bytes_billed` を設定する。Sandbox の無料枠上限、元テーブル・View の最大60日の期限は維持する。上限や期限の変更は含めない。

## 実環境での適用・検証結果

2026-10-15 にユーザーが Raw の履歴本文を含む公開範囲と適用を了承し、`qlpt2020@gmail.com` の認証で Terraform を適用した。プロジェクトは `go-drive-etl-507314`、アプリ専用アカウントは `reflection-documents-app@go-drive-etl-507314.iam.gserviceaccount.com` である。

当初の検証者なしの plan は追加6件・変更0件・削除0件だった。実行時に未有効だった IAM Service Account Credentials API・Policy Troubleshooter API と、一時的な検証者の Token Creator 権限を追加し、最終 plan と apply は追加9件・変更0件・削除0件で成功した。ETL・BI の IAM を変更する差分はなかった。検証者は共有 tfvars を変更せず `-var` で指定した。

適用直後の短期トークン取得は `iam.serviceAccounts.getAccessToken` の権限エラーになった。SA の IAM 設定を読み取り、検証者への付与を確認した。その後、IAM の再適用なしで再試行すると成功したため、権限の反映待ちと整合する。

`./scripts/verify-reflection-app` で `@month = '2026-10'` の dry run と本クエリが成功した。クエリ費用上限は256 MiBである。別の `SELECT SESSION_USER()` でも、実行主体がアプリ専用アカウントに一致した。ユーザー自身のデータ参照権限による成功とは区別できている。

| 集計 | 件数 |
|---|---|
| 月内文書数・文書 ID 一意数 | 各1 |
| 空でない本文数・空でない出典 URL 数 | 各1 |
| 月内の文書名 NULL 数 | 0 |
| 月内の期間開始 NULL 数・終了 NULL 数 | 各1 |
| View 全体の年月 NULL 数 | 0 |

Policy Troubleshooter v3 を検証者の認証で呼び、以下の36件がすべて期待どおりだった。最初の確認では22件が成功し、14件が HTTP 429 になったため、成功済みの項目は繰り返さず、残りだけ5秒間隔で再試行した。API の呼び出し失敗を拒否成功とは扱っていない。

| 主体・対象 | 判定 |
|---|---|
| アプリ → 4リソースの `getData` | 4件すべて `CAN_ACCESS` |
| アプリ → 4リソースの `updateData` / `update` / `delete` / `setIamPolicy` | 16件すべて `CANNOT_ACCESS` |
| アプリ → 3 dataset の `tables.create` / `datasets.update` / `datasets.delete` | 9件すべて `CANNOT_ACCESS` |
| アプリ → Raw `drive_files` の `getData` | 1件 `CANNOT_ACCESS` |
| BI → Raw `chunks` / `chunks_current`・Silver `silver_chunks` の `getData` | 3件すべて `CANNOT_ACCESS` |
| BI → Gold View の `getData`、アプリ・BI → `jobs.create` | 3件すべて `CAN_ACCESS` |

権限確認では実データの更新・削除を行っていない。BI の Gold View 自体への読み取り権限は維持されているが、上流の読み取り権限がないため本文へのアクセスは広がらない。CSV 配信と ETL の設定は変更していない。

検証後、検証者の一時的な Token Creator 権限だけを削除する plan（追加0件・変更0件・削除1件）を確認し、apply も同じ件数で成功した。SA の IAM ポリシーにも検証者の binding が残っていないことを確認し、最後の plan は `No changes` だった。以後の再検証では検証者を明示して再付与する必要がある。発行済み短期トークンは権限削除だけで失効するものとして扱わず、有効期限までの利用可能性を残す。

実文書の本文・出典の内容自体の正しさ、他の月、外部 Python / FastAPI の実行基盤の認証は未検証である。実行場所・外部 IdP が未確定のため、アタッチまたは WIF の設定は本変更に含めない。鍵は発行していない。元テーブル・View の最大60日の期限は維持している。

plan には既存 Gold View の `labels = {}` と `resource_tags = {}` の差異の通知があったが、変更アクションはなかった。共有 tfvars の既存の未宣言変数 `scheduled_query_service_account_email` の警告は残っている。本変更では共有設定を変更していない。

Terraform の静的検証、検証スクリプトのオフラインテスト7件、`git diff --check` は成功した。オフラインテストは `python3 scripts/test-verify-reflection-app` で再実行でき、CI にも追加した。文書の lint も成功した。詳細ログはローカルの `/tmp/issue-137-*.log` と `/tmp/issue-137-effective-iam*.json` に保存し、実文書本文・識別情報・認証情報は記録していない。
