---
name: gh-pr
description: GitHub Pull Request を作成する。ユーザーが「gh-pr」「PRを作って」「プルリクエスト作成して」などと言ったときに使う。
---

# gh-pr スキル

`gh pr create` を使って GitHub Pull Request を作成する。

## 手順

### 1. 現在のブランチと差分を確認する

```bash
git branch --show-current
git log --oneline main..HEAD
git diff main...HEAD --stat
```

### 2. 関連 Issue を確認する

```bash
gh issue list --repo qei-2027-700/go-drive-etl --state open
```

### 3. PR を作成する

```bash
gh pr create \
  --repo qei-2027-700/go-drive-etl \
  --title "<タイトル>" \
  --body "$(cat <<'EOF'
## Summary
- <変更内容を箇条書き>

## Closes
Closes #<Issue番号>

## Test plan
- [ ] <確認事項1>
- [ ] <確認事項2>
EOF
)"
```

## タイトルの形式

Conventional Commits に従う:

| prefix | 用途 |
|---|---|
| `feat:` | 新機能 |
| `fix:` | バグ修正 |
| `chore:` | 雑務・依存更新 |
| `docs:` | ドキュメント |
| `refactor:` | リファクタ |

## 注意

- `Closes #<番号>` を body に含めるとマージ時に Issue が自動クローズされる
- ブランチが remote に push されていない場合は先に `git push -u origin <branch>` を実行する
- draft にしたい場合は `--draft` フラグを追加する
