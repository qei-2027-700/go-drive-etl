---
name: git-commit
description: 変更をステージしてConventional Commits形式でコミットする。ユーザーが「git-commit」「コミットして」「変更をコミット」などと言ったときに使う。
argument-hint: "[issue-number]"
allowed-tools: Bash, Read
---

# git-commit スキル

**まず `docs/workflows/git.md` の「2. 変更のコミット」セクションを Read ツールで読んでから、記載された手順に従って実行してください。**

このスキルは、作業ツリーの変更内容を確認し、秘密情報の混入がないかをチェックした上で、Conventional Commits形式でコミットを作成します。コミットタイプ一覧・secrets チェックコマンド・禁止事項はそのドキュメントに記載されています。
