---
name: marp-pdf
description: Marp形式のMarkdownスライドをPDFへ変換する。ユーザーが「MarpをPDFにして」「スライドをPDF化して」と依頼したときに使う。
---

# Marp PDF

Marp形式のMarkdownを、指定された出力先または入力ファイルと同じ場所のPDFへ変換する。

1. 入力が指定されていなければ、作業対象から `marp: true` を含むMarkdownを候補として示し、対象を確認する。
2. `marp` の利用可否を確認する。未導入なら、導入方法を案内し、インストールはユーザーの承認後に行う。
3. ローカル画像や背景を利用する場合だけ `--allow-local-files` を付けて変換する。

```bash
marp <input.md> --pdf --output <output.pdf> [--allow-local-files]
```

変換後は出力ファイルの存在を確認してパスを報告する。日本語フォントの見た目が重要な場合は、PDFをレンダリングして確認する。
