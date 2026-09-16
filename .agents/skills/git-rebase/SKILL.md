---
name: git-rebase
description: Unpublished commits を interactive rebase で整理する。ユーザーが「rebaseでまとめて」「3つ目以降をfixupして」と依頼したときに使う。
---

# Git rebase

未公開コミットだけを interactive rebase で整理する。

## 安全確認

1. 現在のブランチ、upstream、コミット列を確認する。
2. upstream に含まれるコミットを書き換えない。公開済みの履歴を変更する必要がある場合は、影響と `--force-with-lease` が必要になることを説明してユーザーの明示的な承認を得る。
3. 作業ツリーに未コミットの変更がある場合は中断する。stash や commit を勝手に行わない。

## 3つ目以降を fixup する依頼

コミットが3件以上あり、対象が未公開であることを確認してから実行する。ルートコミットは対象外とし、2件目を残して3件目以降を直前のコミットに `fixup` する。

```bash
GIT_SEQUENCE_EDITOR='perl -i -pe "s/^pick/fixup/ if $. >= 3"' \
  git rebase -i "$(git rev-list --max-parents=0 HEAD)"
```

競合した場合は解消手順を提示し、継続・中止はユーザーの指示に従う。中止する場合は `git rebase --abort` を使う。

## 完了確認

`git log --oneline` と `git status` を確認し、書き換えたコミット数と push が必要かを報告する。
