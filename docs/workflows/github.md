# GitHub 連携ワークフロー

このドキュメントは、GitHub を介した開発サイクル（Issue 起票、PR 作成、レビュー、マージ）の標準手順とルールをまとめたものです。

---

## 1. Issue の起票 (`gh-issue`)

`gh issue create` を使用して GitHub Issue を起票します。

### 手順

1.  **既存ラベルの確認**  
    プロジェクトで現在定義されているラベルを確認します。
    ```bash
    gh label list --repo qei-2027-700/go-drive-etl
    ```

2.  **Issue の作成**  
    ```bash
    gh issue create \
      --repo qei-2027-700/go-drive-etl \
      --title "<タイトル>" \
      --body "$(cat <<'EOF'
    ## 概要
    <何をするか>

    ## 完了条件
    - [ ] <条件1>
    - [ ] <条件2>

    ## 関連ファイル
    <関連するファイルやディレクトリ>
    EOF
    )" \
      --label "<ラベル>"
    ```

### ラベルの分類基準

| ラベル | 用途 |
|---|---|
| `enhancement` | 新機能の開発、既存機能の改善や拡張 |
| `bug` | 不具合、バグ修正 |
| `chore` | リファクタリング、ライブラリの依存関係更新、設定変更など |
| `documentation` | ドキュメントの整備 |

### 注意事項
*   タイトルは動詞から始めてください（例: `Implement Worker Pool`, `Fix ListPending bug`）。
*   ラベルが存在しない場合は、`gh label create --repo qei-2027-700/go-drive-etl` で先に作成してから起票してください。

### 依存関係の表記

Issue 本文に次の定型行を置く。GitHub の Issue 番号を使い、依存がない場合も `None` を明記する。

```txt
Blocked by: #123, #456
Blocks: #789
```

`Blocked by` は着手・完了の前提となる Issue、`Blocks` はこの Issue の完了を待つ後続 Issue を示す。依存の追加・解消時は両側の Issue を更新する。

---

## 2. Pull Request の作成 (`gh-pr`)

`gh pr create` を使用して Pull Request (PR) を作成します。

### 手順

1.  **現在の状況確認**  
    ブランチの確認と、`main` からの差分を確認します。
    ```bash
    git branch --show-current
    ```
    ```bash
    git log --oneline main..HEAD
    ```
    ```bash
    git diff main...HEAD --stat
    ```

2.  **関連 Issue の確認**  
    クローズ対象となる Issue 番号を確認します。
    ```bash
    gh issue list --repo qei-2027-700/go-drive-etl --state open
    ```

3.  **リモートへのプッシュ**  
    事前にローカルコミットをリモートブランチへプッシュしておきます。
    ```bash
    git push -u origin <ブランチ名>
    ```

4.  **PR の作成**  
    ```bash
    gh pr create \
      --repo qei-2027-700/go-drive-etl \
      --title "<タイトル>" \
      --body "$(cat <<'EOF'
    > 🤖 Created via skill `/gh-pr`

    ## Summary
    - <変更内容を箇条書きで簡潔に記載>

    ## Closes
    Closes #<Issue番号>

    ## Test plan
    - [ ] <確認事項1>
    - [ ] <確認事項2>
    EOF
    )"
    ```

### タイトル形式 (Conventional Commits)

PRタイトルは Conventional Commits 形式に従います：
*   `feat: <タイトル>` (新機能)
*   `fix: <タイトル>` (バグ修正)
*   `chore: <タイトル>` (雑務・依存関係更新)
*   `docs: <タイトル>` (ドキュメント)
*   `refactor: <タイトル>` (リファクタ)

### マージ条件

PR は次のすべてを満たしてからマージする。

- CI がすべて green である
- 変更者または別のレビュアーが最終差分をレビューした
- 関連 Issue の完了条件を満たし、PR 本文に `Closes #<Issue番号>` がある

---

## 3. Pull Request のレビュー (`gh-rv`)

PR をレビューし、問題点や改善点をレビューコメントとして残します。

### 手順

1.  **PR 概要の確認**  
    ```bash
    gh pr view <PR番号またはURL> --repo qei-2027-700/go-drive-etl
    ```

2.  **差分の確認**  
    ```bash
    gh pr diff <PR番号またはURL> --repo qei-2027-700/go-drive-etl
    ```

3.  **CI ステータスの確認**  
    ```bash
    gh pr checks <PR番号またはURL> --repo qei-2027-700/go-drive-etl
    ```

4.  **レビューの送信**  
    本文を `--body "..."` やコマンド置換で渡さない。バッククォートを含む Markdown が shell interpolation で変化するため、必ずファイルとして作成し、`scripts/gh-pr-review` を使う。このヘルパーは GitHub Reviews API に本文を raw file として渡し、作成されたレビューの API 応答が元ファイルと完全一致することを検証する。

    ```bash
    cat > /tmp/pr-review.md <<'EOF'
    ## 結論
    変更が必要です。

    ## 指摘
    - **重要度:** high
    - **対象:** `internal/example.go:42`
    - **理由:** `recordID` が未設定のまま保存されます。詳細: https://example.com/design
    - **推奨対応:** 保存前に `recordID` を検証してください。
    EOF

    scripts/gh-pr-review <PR番号> REQUEST_CHANGES /tmp/pr-review.md qei-2027-700/go-drive-etl
    ```

    イベントには `COMMENT`、`APPROVE`、`REQUEST_CHANGES` を指定する。承認も根拠を残す場合は同じテンプレートを使い、本文なしの承認だけは `gh pr review <PR番号> --approve` を使用してよい。

    ヘルパーを使えない状況でも、同じ原則で `gh api --input <json-file>` を使う。本文をシェルの二重引用符、変数展開、コマンド置換に渡してはいけない。

### レビュー本文テンプレート

レビュー本文は、問題の有無にかかわらず次の項目をこの順序で含める。

```md
## 結論
<承認 / 変更が必要 / コメントのみ>

## 指摘
- **重要度:** <critical | high | medium | low | n/a>
- **対象:** `<ファイル名:行番号>` または `<PR全体>`
- **理由:** <なぜ問題または判断材料になるか>
- **推奨対応:** <具体的な変更、または対応不要の理由>
```

複数の指摘がある場合は、`## 指摘 1`、`## 指摘 2` のように見出しを増やし、それぞれに全項目を記載する。

### 投稿後の検証と訂正

`scripts/gh-pr-review` が `body verified` を表示したことを確認する。必要に応じて次の API 取得でも最新レビューを確認できる。

```bash
gh api repos/qei-2027-700/go-drive-etl/pulls/<PR番号>/reviews \
  --jq '.[0] | {id, state, body}'
```

訂正が必要でも、同じ内容を新しいレビューとして投稿しない。最初に既存レビューの ID と誤りを確認し、GitHub の UI または API で既存のレビューを編集・削除できる場合はそれを更新する。更新できない場合は、重複ではなく一件の明確な訂正コメントで対象レビュー ID を参照し、以後の判断は訂正版だけを参照することを明記する。

### レビュー観点
1.  **正確性** — ロジックエラー、境界値処理、リソースリークの有無。
2.  **セキュリティ** — 機密情報の混入、インジェクション対策、Actions等の依存関係ハッシュのピン留め。
3.  **可読性** — 適切な命名、コードの簡潔性、不要なコメントの排除。
4.  **テスト** — 変更に対して十分なユニットテスト等が書かれているか。

---

## 4. Pull Request のマージと後片付け (`gh-merge`)

CI ステータスを確認し、PR を Squash Merge して作業領域（ブランチ・Worktree）を削除します。

### 手順

1.  **PR 情報とブランチ名の確認**  
    ```bash
    gh pr view --repo qei-2027-700/go-drive-etl --json number,headRefName -q '"PR #\(.number) [\(.headRefName)]"'
    ```

2.  **CI の結果確認**  
    **全ての CI が green (pass) になっていることを必ず確認**します。
    ```bash
    gh pr checks --repo qei-2027-700/go-drive-etl
    ```
    *   `fail` がある場合はマージを即座に中断し、原因を調査してユーザーに報告します。

3.  **Squash Merge の実行**  
    ```bash
    gh pr merge --repo qei-2027-700/go-drive-etl --squash --delete-branch
    ```

4.  **Worktree の削除 (Worktree を使用していた場合のみ)**  
    ※ worktree 内から自分自身のディレクトリを削除することはできないため、**必ずメインの作業ディレクトリに戻ってから**実行します。
    ```bash
    cd /Users/km/dev/_github/go-drive-etl
    git worktree remove ../go-drive-etl-feature-<Issue番号> --force
    ```

5.  **ローカル環境の同期とクリーンアップ**  
    ```bash
    git checkout main
    git branch -d <削除したブランチ名>
    git fetch origin --prune
    git pull origin main
    ```
