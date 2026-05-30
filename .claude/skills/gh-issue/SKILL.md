---
name: gh-issue
description: GitHub Issue を起票する。ユーザーが「gh-issue」「issueを作って」「issue起票して」などと言ったときに使う。
---

# gh-issue スキル

`gh issue create` を使って GitHub Issue を起票する。

## 手順

### 1. 既存ラベルを確認する

```bash
gh label list --repo qei-2027-700/go-drive-etl
```

### 2. Issue を起票する

```bash
gh issue create \
  --repo qei-2027-700/go-drive-etl \
  --title "<タイトル>" \
  --body "$(cat <<'EOF'
## 概要
<何をするか>

## 完了条件
- [ ] <条件1>
- [ ] <条件2>

## 関連ファイル
<関連するファイルやディレクトリ>
EOF
)" \
  --label "<ラベル>"
```

## ラベルの選び方

| ラベル | 用途 |
|---|---|
| `enhancement` | 新機能・改善 |
| `bug` | バグ修正 |
| `chore` | リファクタ・依存更新・設定変更 |
| `documentation` | ドキュメント |

## 注意

- タイトルは動詞から始める（例: `Implement Worker Pool`, `Fix ListPending bug`）
- 完了条件は PR マージ時に `Closes #<番号>` で自動クローズされる前提で書く
- ラベルが存在しない場合は `gh label create --repo qei-2027-700/go-drive-etl` で先に作成する
