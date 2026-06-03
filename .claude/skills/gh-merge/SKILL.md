---
name: gh-merge
description: PRのCIが通ったらsquash mergeしてworktreeとブランチを削除する。ユーザーが「gh-merge」「マージして」「PRをマージして」などと言ったときに使う。
argument-hint: <pr-number>
allowed-tools: Bash
---

# gh-merge スキル

CI が全て green であることを確認してから PR を squash merge し、worktree とブランチを後片付けする。

## 手順

### 1. PR 番号を確認する

```bash
gh pr view --repo qei-2027-700/go-drive-etl --json number,headRefName -q '"PR #\(.number) [\(.headRefName)]"'
```

### 2. CI ステータスを確認する

```bash
gh pr checks --repo qei-2027-700/go-drive-etl
```

全て `pass` になるまで待機する。`fail` があれば原因を調査してユーザーに報告し、マージを中断する。

### 3. squash merge を実行する

```bash
gh pr merge --repo qei-2027-700/go-drive-etl --squash --delete-branch
```

### 4. worktree を削除する

```bash
# main ブランチのツリーに戻ってから削除
cd /Users/km/dev/_github/go-drive-etl
git worktree remove ../go-drive-etl-feature-<N> --force
git fetch origin --prune
```

### 5. main を最新化する

```bash
git pull origin main
```

### 6. 完了を報告する

```
マージ完了:
  PR     : #<N>
  Branch : feature/issue-<N>-<slug> (削除済み)
  worktree: ../go-drive-etl-feature-<N>/ (削除済み)
```

## 注意

- **禁止**: CI が `fail` の状態でのマージ。原因をユーザーに伝えて修正を促す
- `--delete-branch` でリモートブランチも自動削除される
- worktree 削除は `/Users/km/dev/_github/go-drive-etl/` で実行する（worktree 内からは削除できない）
