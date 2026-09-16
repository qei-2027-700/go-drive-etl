# スキル運用

Claude Code と Codex で共有するリポジトリ固有のスキルは `.agents/skills/` を正本とする。Claude Code 側では `.claude/skills` がこのディレクトリを指す既存のシンボリックリンクを利用する。Codex は `.agents/skills/` を直接検出するため、Codex用のリンクは作成しない。

`.claude/` と `CLAUDE.md` は Claude Code 用として維持する。Codex 固有の永続指示は `AGENTS.md`、個人専用スキルは `~/.codex/skills/` に置く。

## 移行対応表

| Claude Codeの既存スキル | Codexでの扱い | 正本 |
| --- | --- | --- |
| `gh-issue`, `gh-merge`, `gh-pr`, `gh-rv`, `gh-worktree` | 共有済み | `.agents/skills/` |
| `git-commit`, `git-push` | 共有済み | `.agents/skills/` |
| `git-rebase` | 共有スキルとして移植。公開済み履歴の書換えには明示承認が必要 | `.agents/skills/git-rebase/` |
| `marp-pdf` | 共有スキルとして移植 | `.agents/skills/marp-pdf/` |
| `transcribe` | 共有スキルとして移植 | `.agents/skills/transcribe/` |
| `mermaid` | Codex組み込みの `visualize` スキルで代替。Mermaidコードの出力や保存が必要な場合は、その旨を依頼に含める | Codex `visualize` |

同じ名前のスキルを個人スキルとリポジトリスキルの両方に置かない。リポジトリ固有のワークフローはこの対応表と `.agents/skills/` を更新する。
