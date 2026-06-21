# Git 作業ワークフロー

このドキュメントは、本リポジトリにおける Git 操作（コミット、プッシュ、ワークツリー管理）の標準手順とルールをまとめたものです。

---

## 1. ワークツリー管理 (`gh-wt`)

新しい作業ブランチを作成して並列で開発を行う場合、Git Worktree 機能を使用します。

### 手順

1.  **事前準備**  
    `main` ブランチの最新コミットから分岐させるため、`main` を最新化します。
    ```bash
    git fetch origin && git merge origin/main
    ```

2.  **Issue 情報の確認**  
    対象の Issue のタイトルを取得し、ブランチ名用のスラッグを検討します。
    ```bash
    gh issue view <Issue番号> --repo qei-2027-700/go-drive-etl --json number,title -q '"#\(.number) \(.title)"'
    ```

3.  **スラッグとブランチ名の決定**  
    タイトルから英小文字・ハイフン区切りのスラッグを生成します。
    *   例: `Implement Worker Pool with graceful shutdown` → `worker-pool`
    *   ブランチ名形式: `feature/issue-<Issue番号>-<slug>`
    *   例: `feature/issue-5-worker-pool`

4.  **Worktree の追加**  
    リポジトリルートの 1 つ上の親ディレクトリに作業用ディレクトリを作成します。
    ```bash
    git worktree add ../go-drive-etl-feature-<Issue番号> -b feature/issue-<Issue番号>-<slug>
    ```

5.  **作成の確認**  
    ```bash
    git worktree list
    ```

### 注意事項
*   同名のローカルブランチが既に存在する場合は、`-b` フラグを除外して既存のブランチを指定します。

---

## 2. 変更のコミット (`git-commit`)

変更をステージし、Conventional Commits 形式に従ってコミットを作成します。

### 手順

1.  **変更内容の確認**  
    作業ディレクトリ内の変更差分とステータスを確認します。
    ```bash
    git status
    ```
    ```bash
    git diff --stat
    ```

2.  **秘密情報の混入チェック (Secrets Check)**  
    以下のパターンに一致する機密ファイルがステージ対象（または作業領域）に含まれていないか確認します。**混入が疑われる場合はコミットを即座に中断**し、ユーザーに報告します。
    ```bash
    git status --short | grep -E '\.env|\.pem|\.key|credentials|secret|token'
    ```

3.  **検証の実行**  
    ビルドや静的解析が通っているか確認します。
    *   **禁止**: `go build ./...` が失敗している状態でのコミット

4.  **変更のステージ**  
    ```bash
    git add -A
    ```

5.  **コミットの作成**  
    Conventional Commits 形式でコミットメッセージを作成します。
    ```bash
    git commit -m "$(cat <<'EOF'
    <type>(<scope>): <subject>

    <body（任意）>

    Generated-by: `git-commit` skill
    EOF
    )"
    ```
    *   Issue を閉じる場合は、本文（body）に `Closes #<N>` を含めてください。

### コミットメッセージのタイプ (`type`)

| type | 用途 |
|---|---|
| `feat` | 新機能の開発 |
| `fix` | バグ修正 |
| `chore` | リファクタリングに含まれない雑務、依存関係の更新、設定変更など |
| `docs` | ドキュメントの追加・修正 |
| `refactor` | 仕様変更のないリファクタリング |
| `test` | テストの追加・修正 |
| `ci` | CI/CD 設定（GitHub Actions 等）の変更 |

### 注意事項
*   1コミット1論理変更を厳守してください。

---

## 3. リモートへのプッシュ (`git-push`)

コミットした内容をリモートの `origin` に安全に送信します。

### 手順

1.  **現在のブランチの確認**  
    ```bash
    git branch --show-current
    ```

2.  **リモート追跡ブランチの有無を確認**  
    ```bash
    git status -sb | head -1
    ```

3.  **プッシュの実行**  
    *   **新規ブランチ（追跡ブランチ未設定）の場合:**
        ```bash
        git push -u origin <ブランチ名>
        ```
    *   **追跡ブランチ設定済みの場合:**
        ```bash
        git push
        ```

4.  **プッシュ前の確認**  
    プッシュ前に、まだリモートに送っていないコミットの概要を確認することをおすすめします。
    ```bash
    git log --oneline origin/<ブランチ名>..HEAD
    ```

### 禁止事項
*   `main` / `master` ブランチへの `--force` および `--force-with-lease` プッシュは**厳禁**です。
