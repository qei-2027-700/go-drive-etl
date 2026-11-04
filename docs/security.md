# Supply Chain Security ルール

## GitHub Actions

- アクションは必ずコミット SHA でピン留めする（タグ指定禁止）
- バージョンはコメントで明示する: `uses: actions/foo@<sha> # v1.2.3`
- 使用前に必ず最新バージョンを Web で確認し、古いバージョンを使わない
- `permissions` は最小権限を明示する（`read-all` または個別指定）

```yaml
# NG
uses: actions/checkout@v4

# OK
uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
```

## 依存ライブラリ

- `go.sum` は必ずコミットに含める（チェックサム保証）
- `go install xxx@latest` は CI では避け、バージョンを固定する
- Dependabot で月次の脆弱性チェックを有効化済み

## サービスアカウントの JSON キー

Drive API は OAuth の3設定がそろうとユーザー認証を優先し、それ以外は ADC を使う。サービスアカウントの JSON キーを使う場合は、パスワード同等の機密情報として扱う（[#73](https://github.com/qei-2027-700/go-drive-etl/issues/73)）。

- JSON キーはリポジトリ外に保存する。リポジトリ内に置く場合は `secrets/` 配下に限定し、`.gitignore` で除外する（誤コミット防止の多重防御であり、リポジトリ外保存が原則）
- パスは環境変数 `GOOGLE_APPLICATION_CREDENTIALS` で渡し、コードやコミットに直接埋め込まない
- 現在の Drive クライアントは入力の読み取りと Gold CSV の配信に共通の `DriveScope` を要求する。入力だけを扱うサービスアカウントには対象フォルダの閲覧者権限を付け、配信に使う認証主体には出力先への書き込み権限を別途確認する
- キーが漏洩した疑いがある場合は、GCP コンソールのサービスアカウント「キー」タブから該当キーを即座に削除し、新しいキーを再発行する
- 将来 Cloud Run 等にデプロイする際は、JSON キーファイル自体が不要になる Workload Identity への移行を検討する
