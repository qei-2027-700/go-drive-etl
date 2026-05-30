# /feature コマンド

Issue 番号を受け取り、ブランチ作成から実装・PR・マージまでの開発フローを自動実行する。

**引数**: `<issue番号> [--manual]`

例:
- `/feature 5`          → 全自動（Claude が実装まで行う）
- `/feature 5 --manual` → worktree 作成後に停止し、ユーザーが実装する

## 使用するスキル

各ステップで以下のスキルを `Skill` ツール経由で呼び出す。直接実装せずスキルに委譲すること。

| ステップ | スキル名 |
|---|---|
| Step 3 | `gh-wt` |
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

### Step 3: worktree 作成

`Skill` ツールで `gh-wt` を呼び出し、Issue 番号とタイトルから `feature/issue-<N>-<slug>` ブランチと worktree を作成する。

### Step 4: 実装

`--manual` フラグがある場合:
- 「worktree が `../go-drive-etl-feature-<N>/` に作成されました。実装が完了したら `/feature-continue` を実行してください。」と表示して**停止する**。

`--manual` フラグがない場合:
- Issue の完了条件を元に Claude が実装を行う。
- worktree 内のディレクトリで作業する。
- `go build ./...` でビルドが通ることを確認する。

### Step 5: コミット＆プッシュ

1. `Skill` ツールで `git-commit` を呼び出す（body に `Closes #<N>` を含める）。
2. `Skill` ツールで `git-push` を呼び出す。

### Step 6: PR 作成

`Skill` ツールで `gh-pr` を呼び出し、`Closes #<N>` を body に含めた PR を作成する。

### Step 7: CI 待機

```bash
gh run watch --repo qei-2027-700/go-drive-etl
```

CI が完了するまで待機する。

### Step 8: セルフレビュー

`Skill` ツールで `gh-rv` を呼び出し、PR の差分をレビューする。問題がなければ approve する。

### Step 9: マージ

`Skill` ツールで `gh-merge` を呼び出し、squash merge を実行し worktree とブランチを削除する。
