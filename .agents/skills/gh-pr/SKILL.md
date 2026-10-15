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

PR 本文は次の形式で始めます。レビュアーが冒頭だけで目的を理解できるよう、Issue と差分で確認した課題と、変更後にどうなるかを 1〜3 文で書いてください。変更ファイルや作業の列挙だけで済ませず、未確認の効果は補わないでください。

```markdown
> [!IMPORTANT]
> <解決する課題と、変更後にどうなるかを 1〜3 文で記載>

## 変更内容
...
```

IMPORTANT アラートは既存の概要や生成元の表記より前に置きます。詳細な変更内容・関連 Issue・検証結果・必要なスクリーンショットなどの後続セクションは残し、生成元の表記は本文の末尾に置いてください。
