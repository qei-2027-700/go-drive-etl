# Issue 着手時のモデル選択ルール

issue に着手するときは、まず `gh issue view <番号>` でラベルを確認する。

- issue に `model:haiku` / `model:sonnet` / `model:opus` ラベルが付いている場合、現在のセッションのモデルがそれと一致しているかを確認する
- 不一致の場合は作業を始める前にユーザーへ知らせ、`/model` での切り替え（またはセッションの `claude --model <名前>` での起動し直し）を促す
- ラベルの目安: haiku = docs・定型 chore / sonnet = 通常の実装・テスト / opus = 設計判断を伴うタスク
