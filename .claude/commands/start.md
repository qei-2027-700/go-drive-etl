# /start コマンド

Issue 番号を受け取り、worktree を作成せずに現在のディレクトリでブランチを切って開発フローを進める。
`/feature` との違いは Step 3 のみ（worktree なし・直接ブランチ作成）。

**引数**: `<issue番号>`

例:
- `/start 5` → ブランチ作成後に停止し、ユーザーが実装する

## 使用するスキル

各ステップで以下のスキルを `Skill` ツール経由で呼び出す。直接実装せずスキルに委譲すること。

| ステップ | スキル名 |
|---|---|
| Step 5 | `git-commit` → `git-push` |
| Step 6 | `gh-pr` |
| Step 8 | `gh-rv` |
| Step 9 | `gh-merge` |

## 実行ステップ

以下のステップを順番に実行してください。

### Step 1: Issue 確認

```bash
gh issue view $ISSUE_NUMBER --repo qei-2027-700/go-drive-etl
```

Issue のタイトル・概要・完了条件を表示してユーザーに確認する。

### Step 2: main を最新化

```bash
git fetch origin
git merge origin/main
```

### Step 3: ブランチ作成

Issue 番号とタイトルから `feature/issue-<N>-<slug>` ブランチを現在のディレクトリに作成する。

```bash
gh issue view $ISSUE_NUMBER --repo qei-2027-700/go-drive-etl --json number,title -q '"#\(.number) \(.title)"'
```

タイトルからスラッグを生成し、ブランチを作成する：

```bash
git checkout -b feature/issue-<N>-<slug>
```

### Step 4: 実装待機

「ブランチ `feature/issue-<N>-<slug>` を作成しました。実装が完了したら `/start-continue` を実行してください。」と表示して**停止する**。

### Step 5: コミット＆プッシュ

1. `Skill` ツールで `git-commit` を呼び出す（body に `Closes #<N>` を含める）。
2. `Skill` ツールで `git-push` を呼び出す。

### Step 6: PR 作成

`Skill` ツールで `gh-pr` を呼び出し、`Closes #<N>` を body に含めた PR を作成する。

### Step 7: CI 待機

```bash
gh run list --repo qei-2027-700/go-drive-etl --limit 1 --json databaseId -q '.[0].databaseId'
gh run watch <run-id> --repo qei-2027-700/go-drive-etl --exit-status
```

CI が完了するまで待機する。

### Step 8: セルフレビュー

`Skill` ツールで `gh-rv` を呼び出し、PR の差分をレビューする。問題がなければ approve する。

### Step 9: マージ

`Skill` ツールで `gh-merge` を呼び出し、squash merge を実行してブランチを削除する。
