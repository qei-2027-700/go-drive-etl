---
name: gh-wt
description: Issue番号からfeatureブランチとgit worktreeを作成する。ユーザーが「gh-wt」「worktreeを作って」「ブランチとworktreeを切って」などと言ったときに使う。
---

# gh-wt スキル

Issue 番号とタイトルから `feature/issue-<N>-<slug>` ブランチを作成し、`git worktree add` で並列作業用のワーキングツリーを作る。

## 手順

### 1. Issue 情報を取得する

```bash
gh issue view <N> --repo qei-2027-700/go-drive-etl --json number,title -q '"#\(.number) \(.title)"'
```

### 2. スラッグを生成する

タイトルから英小文字・ハイフン区切りのスラッグを生成する。
例: `Implement Worker Pool with graceful shutdown` → `worker-pool`

ブランチ名: `feature/issue-<N>-<slug>`
例: `feature/issue-5-worker-pool`

### 3. worktree を作成する

```bash
# リポジトリルートから1つ上のディレクトリに作成
git worktree add ../go-drive-etl-feature-<N> -b feature/issue-<N>-<slug>
```

例:
```bash
git worktree add ../go-drive-etl-feature-5 -b feature/issue-5-worker-pool
```

### 4. 作成を確認する

```bash
git worktree list
```

### 5. worktree のパスを報告する

```
worktree 作成完了:
  パス   : ../go-drive-etl-feature-<N>/
  ブランチ: feature/issue-<N>-<slug>
```

## 注意

- worktree は main の最新コミットから分岐する
- 作成前に `git fetch origin && git merge origin/main` で main を最新にしておく
- 同名ブランチが既に存在する場合は `-b` を外して既存ブランチを使う
