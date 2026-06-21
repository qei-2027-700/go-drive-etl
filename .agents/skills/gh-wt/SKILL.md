---
name: gh-wt
description: Issue番号からfeatureブランチとgit worktreeを作成する。ユーザーが「gh-wt」「worktreeを作って」「ブランチとworktreeを切って」などと言ったときに使う。
argument-hint: <issue-number>
allowed-tools: Bash, Read
---

# gh-wt スキル

**まず `docs/workflows/git.md` の「1. ワークツリー管理」セクションを Read ツールで読んでから、記載された手順に従って実行してください。**

このスキルは、Issue番号に基づいて新規フィーチャーブランチを作成し、並列作業用のGit Worktreeを追加します。ブランチ名スラッグのルール・worktree の作成先パスはそのドキュメントに記載されています。
