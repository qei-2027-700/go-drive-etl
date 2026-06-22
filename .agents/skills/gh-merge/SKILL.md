---
name: gh-merge
description: PRのCIが通ったらsquash mergeしてworktreeとブランチを削除する。ユーザーが「gh-merge」「マージして」「PRをマージして」などと言ったときに使う。
argument-hint: <pr-number>
allowed-tools: Bash, Read
---

# gh-merge スキル

**まず `docs/workflows/github.md` の「4. Pull Request のマージと後片付け」セクションを Read ツールで読んでから、記載された手順に従って実行してください。**

このスキルは、PRのCIチェック結果を確認し、Squash Mergeを行ったあと、ローカルおよびリモートの不要なブランチやWorktreeを削除します。CI fail 時のルールや worktree 削除の注意事項はそのドキュメントに記載されています。
