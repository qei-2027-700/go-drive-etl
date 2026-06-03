---
name: git-push
description: 現在のブランチをリモートにpushする。ユーザーが「git-push」「pushして」「リモートに上げて」などと言ったときに使う。
allowed-tools: Bash
---

# git-push スキル

現在のブランチをリモートの `origin` に push する。

## 手順

### 1. 現在のブランチを確認する

```bash
git branch --show-current
```

### 2. リモート追跡ブランチの有無を確認する

```bash
git status -sb | head -1
```

### 3. push する

追跡ブランチが未設定の場合（新規ブランチ）:

```bash
git push -u origin <branch-name>
```

追跡ブランチが設定済みの場合:

```bash
git push
```

### 4. push 結果を報告する

```
push 完了:
  ブランチ: <branch-name>
  リモート: origin/<branch-name>
```

## 注意

- **禁止**: main/master ブランチへの `--force` および `--force-with-lease`
- push 前に `git log --oneline origin/<branch>..HEAD` でローカル先行コミットを確認する
- CI が自動起動する場合は `gh run list --repo qei-2027-700/go-drive-etl` で確認できる
