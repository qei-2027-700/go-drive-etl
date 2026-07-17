---
name: gh-rv
description: GitHub Pull Request をレビューする。ユーザーが「gh-rv」「PRをレビューして」「このPRを見て」などと言ったときに使う。
argument-hint: <pr-number>
allowed-tools: Bash, Read
# model はセッション継承（レビューは判断を伴うため固定しない）
effort: high
---

# gh-rv スキル

**まず `docs/workflows/github.md` の「3. Pull Request のレビュー」セクションを Read ツールで読んでから、記載された手順に従って実行してください。**

このスキルは、PRの概要・差分・CI状況を確認してレビューを行い、必要に応じて承認または変更要求コメントを送信します。レビュー観点（正確性・セキュリティ・可読性・テスト）の詳細はそのドキュメントに記載されています。
