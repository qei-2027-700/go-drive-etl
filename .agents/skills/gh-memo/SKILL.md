---
name: gh-memo
description: 学習メモをGitHub Issue #26 にコメントとして追記する。ユーザーが「gh-memo」「memoして」「メモして」「issueに記録して」などと言ったときに使う。
allowed-tools: Bash
---

# gh-memo スキル

学習メモ・気づき・教訓を、現在のリポジトリの学習メモ Issue にコメントとして追記する。

## 規約

- メモ Issue は `memo` ラベルが付いた Issue を使う
- Issue は Closed のまま運用する（`is:open` のデフォルト表示に出ないようにするため）

## 手順

### 1. 現在のリポジトリを確認する

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

### 2. メモ Issue を特定する

```bash
gh issue list --repo <nameWithOwner> --label memo --state closed --json number,title -q '.[0] | "#\(.number) \(.title)"'
```

見つからない場合は新規作成する：

```bash
gh label create memo --description "学習メモ・教訓の蓄積" --color "#C5DEF5" --repo <nameWithOwner>
gh issue create --repo <nameWithOwner> \
  --title "学習メモ" \
  --body "開発中に学んだことや気づきを蓄積するissue。コメント欄に随時追記。" \
  --label "memo"
gh issue close <番号> --repo <nameWithOwner>
```

### 3. メモ内容を整形する

ユーザーが伝えた内容を以下の形式に整形する。
冒頭に必ず `> 🗒️ Posted via \`/gh-memo\`` を挿入すること:

```
> 🗒️ Posted via `/gh-memo`

## <タイトル>

### 背景
<どんな状況で気づいたか>

### 気づき・教訓
<学んだこと・正しい理解>

### コード例（あれば）
\`\`\`go
// 例
\`\`\`
```

### 4. コメントを追記する

```bash
gh issue comment <番号> --repo <nameWithOwner> --body "$(cat <<'EOF'
<整形した内容>
EOF
)"
```

### 5. 完了を報告する

```
メモを追記しました:
  Issue: https://github.com/<nameWithOwner>/issues/<番号>
```

## 注意

- Issue は Closed のまま維持する（reopen しない）
- `memo` ラベルで Issue を特定するため、リポジトリをまたいで使える
