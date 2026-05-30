---
name: git-commit
description: 変更をステージしてConventional Commits形式でコミットする。ユーザーが「git-commit」「コミットして」「変更をコミット」などと言ったときに使う。
---

# git-commit スキル

変更をステージして Conventional Commits 形式のメッセージでコミットする。

## 手順

### 1. 変更内容を確認する

```bash
git status
git diff --stat
```

### 2. 変更をステージする

```bash
git add -A
```

### 3. コミットメッセージを作成してコミットする

差分を元に Conventional Commits 形式のメッセージを生成する。

```bash
git commit -m "$(cat <<'EOF'
<type>(<scope>): <subject>

<body（任意）>

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

Issue を閉じる場合は body に `Closes #<N>` を含める。

## コミットタイプ

| type | 用途 |
|---|---|
| `feat` | 新機能 |
| `fix` | バグ修正 |
| `chore` | 雑務・依存更新 |
| `docs` | ドキュメント |
| `refactor` | リファクタ |
| `test` | テスト追加・修正 |
| `ci` | CI/CD 設定 |

## 注意

- `go build ./...` が通っていることを確認してからコミットする
- `.env` や secrets を含むファイルは絶対にステージしない
- 1コミット1論理変更を心がける
