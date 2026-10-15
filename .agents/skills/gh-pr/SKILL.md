---
name: gh-pr
description: GitHub Pull Request を作成する。ユーザーが「gh-pr」「PRを作って」「プルリクエスト作成して」などと言ったときに使う。
argument-hint: "[issue-number]"
allowed-tools: Bash, Read
model: sonnet
effort: medium
---

# gh-pr スキル

**まず `docs/workflows/github.md` の「2. Pull Request の作成」セクションを Read ツールで読んでから、記載された手順に従って実行してください。**

このスキルは、現在の変更をリモートにプッシュし、Pull Requestを作成します。PRテンプレートの構成・タイトルの Conventional Commits ルールはそのドキュメントに記載されています。

PR 本文の先頭は `> [!IMPORTANT]` とし、続く引用行に解決する課題と変更後の状態を1〜2文、100字程度を目安に書きます。Issue・差分で確認した内容に絞り、詳細や Issue リンクは後続の欄に記載します。日本語の本文は `yomiyasu` スキルで推敲し、意味・条件を保って重複や不要な前置きを省きます。
