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
