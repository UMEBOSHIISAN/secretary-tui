<p align="center">
  <img src="assets/logo.svg" alt="secretary-tui" width="720">
</p>

<p align="center">
  <b>The dashboard that cannot press anything.</b><br>
  <sub>秘書の朝刊 — 何も書き換えない・何も承認しない・観測するだけの1画面。</sub>
</p>

<p align="center">
  <img alt="language" src="https://img.shields.io/badge/language-Go%201.26%2B-6FE0A0">
  <img alt="tui" src="https://img.shields.io/badge/tui-bubbletea-8FE7FF">
  <img alt="scope" src="https://img.shields.io/badge/scope-read--only-6E8FE0">
  <img alt="writes" src="https://img.shields.io/badge/writes-zero-e06a6a">
  <img alt="license" src="https://img.shields.io/badge/license-MIT-FFD24D">
</p>

---

xops の投稿キュー、RAG研究の蓄積、ローカルLLM workerの状態を1画面にまとめる、**読み取り専用**のターミナルダッシュボード。

「ログを見に行かない」思想の実装版です。

> A read-only terminal dashboard for a local AI-agent workflow: post queue, research corpus size, and local LLM worker seats on one screen. It writes nothing, approves nothing, and triggers nothing. Every number on it is a snapshot you are expected to distrust before acting on it.

なぜ書き込み機能を持たないのか。**観測画面に承認ボタンを1つ足した瞬間、それは観測画面ではなくなる**からです。ダッシュボードを見ている状態は「まだ判断していない状態」であるべきで、そこに実行経路があると、見ることと決めることが混ざります。

---

## できること

| パネル | 表示内容 | ソース |
|--------|----------|--------|
| xops spool | queued / sending / posted / failed 件数、最終送信時刻 | ローカルの xops spool ディレクトリ |
| RAG research | `active/research/` 配下の記事数 | ローカルの RAG ディレクトリ |
| local LLM workers | alias一覧・backend・host・状態(●緑=ready) | `llm-seat.sh list` |
| AI governance（任意） | WGM handoff または Router dry-run manifest の安全な要約 | `--governance` で明示したローカルJSON |

10秒ごとに自動更新。`r`キーで手動更新、`q`/`Esc`/`Ctrl-C`で終了。

### 実際の動作（GIF）

<p align="center">
  <img src="assets/demo.gif" alt="secretary-tui demo" width="700">
</p>

実行→自動更新(`r`)→終了(`q`)までの一連の流れ（[vhs](https://github.com/charmbracelet/vhs)で録画）。

### テキスト出力例（`--dump`）

```
 秘書の朝刊   — 読み取り専用ダッシュボード（q終了 / r更新）

╭─────────────────────────────────────╮ ╭────────────────╮
│  xops spool                         │ │  RAG research  │
│ queued : 0                          │ │ 記事数: 217 件 │
│ sending: 0                          │ ╰────────────────╯
│ posted : 125                        │
│ failed : 0                          │
│ last   : 2026-07-02T15:57:02.730390 │
╰─────────────────────────────────────╯

╭───────────────────────────────────────╮
│  local LLM workers                    │
│ ● gemma-fast-mini      ollama/mini    │
│ ● gemma-26b-qat-mini   llama.cpp/mini │
│ ● qwen-fast-mini       ollama/mini    │
│ ● embed-mini           ollama/mini    │
│ ...                                    │
╰───────────────────────────────────────╯

最終更新 20:40:15
```

実際の画面はターミナル上でカラー表示されます（緑=queued/ready、水色=research、金=workers）。

---

## ビルド・実行

**Go 1.26 以上**が必要です（`go.mod` 参照）。

```bash
git clone https://github.com/UMEBOSHIISAN/secretary-tui.git
cd secretary-tui
go build -o secretary-tui .
./secretary-tui
```

`go: command not found` になる場合は Go ツールチェーンが入っていないか PATH にありません。Homebrew なら `brew install go`、その後 `export PATH="/opt/homebrew/bin:$PATH"`。

`--dump` フラグで1回だけ描画してプレーンテキスト出力（動作確認・デバッグ・CI用）:

```bash
./secretary-tui --dump
```

### WGM / Mothership Router の観測

[Workflow Governance Model](https://github.com/UMEBOSHIISAN/workflow-governance-model) または
[Mothership Router](https://github.com/UMEBOSHIISAN/mothership-router) の結果を、
**明示したファイルから**読み取り専用で表示できます。

```bash
./secretary-tui --governance ./wgm-handoff.json
./secretary-tui --dump --governance ./router-manifest.json
```

Router manifest 1.0 またはWGM handoff 1.1（1.0互換読取あり）を、dashboard更新・home directory参照・
外部コマンド実行なしで、機械可読な観測JSONへ変換する独立モード:

```bash
./secretary-tui --snapshot-json --governance ./router-manifest.json
```

出力はcompact JSON 1件と改行だけです。summary各行はportable ASCIIで120文字以内に境界化されます。
`--snapshot-json`は`--governance`を必須とし、
`--dump`との併用を入力ファイルを読む前に拒否します。

対応範囲:

| 入力 | 対応バージョン | 表示 |
|------|----------------|------|
| WGM public handoff | WGM 0.2.x / handoff schema 1.1（1.0互換読取） | task ID、capability、risk、token budget、evidence件数 |
| Router manifest | Mothership Router 0.3.x / manifest 1.0 | status、candidate alias、reasons。snapshot export対応 |
| Router manifest（legacy） | Mothership Router 0.2.x / unversioned | dashboard表示のみ。snapshot export不可 |

ファイルは自動探索しません。1 MiBを超えるファイル、壊れたJSON、非対応形式、
秘密情報を示すキー、portable ASCII token grammar外の公開識別子、
またはauthority/execution effectが`true`のmanifestはfail-closedで拒否します。
公開識別子は先頭英数字、以降は英数字・`.`・`_`・`:`・`-`のみで、
drive-relative `X:` prefixは拒否します。
新規連携はportable contractであるWGM 1.1を使用します。1.0入力も認識しますが、
Secretary側のfail-closedなconsumer policyは同様に適用します。
表示はローカルsnapshotであり、承認・実行・鮮度の証明ではありません。

自動探索しないのは不便のためではありません。**探索する画面は、何を読んだかを自分で決める画面**であり、それは読み取り専用の境界の外側です。読むファイルは常にあなたが名前で指定します。

### Mothership conformance

Secretary TUIが`observation-snapshot` 1.0の意味とschemaを所有します。
[`suite/mothership-0.2-conformance.json`](suite/mothership-0.2-conformance.json)は
owner schemaのSHA-256と、credentialを含まない合成例を固定します。
[Mothership 0.2](https://github.com/UMEBOSHIISAN/mothership)はそのowner bytesを
凍結して任意のcompanion chainを検査します。conformanceが証明するのはshape・version・
effectが常にfalseであることだけで、承認・実行・鮮度・remote公開は証明しません。

---

## ⚠️ Safety / Scope

**secretary-tui は観測専用ツールです。承認・実行・意思決定の代替ではありません。**

- ファイルへの書き込み・承認・通知の送信は一切しない
- `--governance` は指定された1ファイルだけを読み、raw JSON や秘密値を表示しない
- `--snapshot-json` は明示されたgovernanceファイル以外を読まず、home / RAG / xopsを参照せず、外部コマンドもtimerも起動しない
- governance 表示は `authority: none` / `execution: none` を常に明示する
- 唯一の外部コマンド実行は `llm-seat.sh list`（worker状態を取得する読み取り専用サブコマンド）のみ
- worker 稼働状況は `llm-seat.sh list` の静的な `ready` 表示。**プロセスの実稼働確認ではない**
- 表示されている数値はキャッシュ/スナップショットの可能性がある。重要な判断の前には元データを直接確認すること
- **このダッシュボードの表示のみを根拠に「稼働している」「停止している」と断定しない**

最後の一行は運用上の実話に基づいています。表示された最終送信時刻を見て「投稿が止まっている」と結論し、実際にはそれが古いスナップショットへのフォールバック値だった、という誤読が実際に起きました。**計器は、それが計器であることを自分で申告しなければならない。** この節はそのための節です。

---

## 構成

```
secretary-tui/
├── main.go             # bubbletea model/update/view 全部（小さいので分割していない）
├── governance.go       # governance JSONの安全な読み取り・要約
├── governance_test.go  # reader・秘密情報境界・表示の単体テスト
├── conformance_test.go # observation schema・CLI隔離・Mothership適合テスト
├── schemas/            # observation-snapshot 1.0 owner schema
├── suite/              # Mothership 0.2 conformance manifest
├── examples/           # 合成Router入力とcanonical observation出力
├── go.mod / go.sum
├── demo.tape           # assets/demo.gif を撮り直すための vhs スクリプト
├── assets/
│   ├── logo.svg
│   └── demo.gif
├── docs/               # governance パネルの設計・実装計画
├── .github/workflows/build.yml
├── LICENSE
├── Plans.md
├── README.md
└── CHANGELOG.md
```

- [bubbletea](https://github.com/charmbracelet/bubbletea) — TUIフレームワーク
- [lipgloss](https://github.com/charmbracelet/lipgloss) — スタイリング

### 前提

- `llm-seat.sh` が存在すること（なくてもクラッシュせず空欄表示）
- xops / RAG のパスが読めること（読み取りのみ、書き込み一切なし）
- **このリポジトリは作者個人のローカル運用パスに依存しています。** 他環境で使う場合は `main.go` 内の `filepath.Join(home, ...)` を自分の環境に合わせて書き換えてください

---

## 関連プロジェクト

同じ「観測して、人間が判断する」境界を共有する部品群です。

| Project | Role |
|---|---|
| [mothership](https://github.com/UMEBOSHIISAN/mothership) | この境界を支える可搬な control plane — 契約・診断・権限境界 |
| [workflow-governance-model](https://github.com/UMEBOSHIISAN/workflow-governance-model) | evidence と authority trail の検証（このTUIが読む側） |
| [mothership-router](https://github.com/UMEBOSHIISAN/mothership-router) | 人間承認境界付きの dry-run routing（このTUIが読む側） |
| [m5-agent-stars](https://github.com/UMEBOSHIISAN/m5-agent-stars) | 同じ思想の物理ディスプレイ版 |
| [claude-cardputer-buddy](https://github.com/UMEBOSHIISAN/claude-cardputer-buddy) | 逆側の端 — 観測ではなく、承認を物理キーで返すデバイス |

このTUIと cardputer-buddy は同じ線の両端です。片方は**絶対に押せない**画面、もう片方は**押すためだけにある**キー。混ぜないことが設計です。

---

## ライセンス

MIT — 遊び・実験用。本番 ops とは独立。
