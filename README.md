# secretary-tui

「秘書の朝刊」— xops投稿キュー・RAG研究の蓄積・ローカルLLM workerの状態を1画面にまとめる、
**読み取り専用**のターミナルダッシュボード。

「ログを見に行かない」思想の実装版。何も書き換えない・何も承認しない・観測するだけ。

---

## できること

| パネル | 表示内容 | ソース |
|--------|----------|--------|
| xops spool | queued / sending / posted / failed 件数、最終送信時刻 | `~/Workspace/Projects/Umeboshi/xops/spool/` |
| RAG research | `active/research/` 配下の記事数 | `~/Workspace/RAG/active/research/` |
| local LLM workers | alias一覧・backend・host・状態(●緑=ready) | `~/Workspace/scripts/llm-seat.sh list` |

10秒ごとに自動更新。`r`キーで手動更新、`q`/`Esc`/`Ctrl-C`で終了。

---

## ビルド・実行

```bash
cd ~/Workspace/遊び枠/secretary-tui
export PATH="/opt/homebrew/bin:$PATH"   # go コマンドが見えない場合
go build -o secretary-tui .
./secretary-tui
```

`--dump` フラグで1回だけ描画してプレーンテキスト出力（動作確認・デバッグ用）:

```bash
./secretary-tui --dump
```

---

## 構成

```
secretary-tui/
├── main.go     # bubbletea model/update/view 全部（小さいので分割していない）
├── go.mod
└── README.md
```

- [bubbletea](https://github.com/charmbracelet/bubbletea) — TUIフレームワーク
- [lipgloss](https://github.com/charmbracelet/lipgloss) — スタイリング

---

## 前提

- `~/Workspace/scripts/llm-seat.sh` が存在すること（なくてもクラッシュせず空欄表示）
- xops/RAGのパスが読めること（読み取りのみ、書き込み一切なし）

---

## スコープ外

- 承認・実行・書き込みは一切しない（観測のみ）
- worker稼働状況は `llm-seat.sh list` の静的な `ready` 表示。プロセスの実稼働確認ではない

---

## ライセンス

MIT — 遊び・実験用。xops / 本番 ops とは独立。
