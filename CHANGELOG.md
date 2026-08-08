# Changelog

このプロジェクトの変更点を記録する。フォーマットは [Keep a Changelog](https://keepachangelog.com/) に準拠。

## [1.1.1] - 2026-08-08

### Changed
- GitHub ActionsをNode 24対応の`actions/checkout@v7`と`actions/setup-go@v7`へ更新

## [1.1.0] - 2026-08-08

### Added
- 明示指定したWGM 1.0 handoffまたはMothership Router 0.2.x dry-run manifestを
  読み取り専用で表示する`AI governance`パネル
- `--governance <JSONファイル>`フラグと`--dump --governance`による検証可能な出力
- governance readerのGo単体テストとGitHub Actionsテスト工程

### Security
- 1 MiB上限、壊れたJSON・非対応形式・秘密情報キーのfail-closed拒否
- `authority_effect`または`execution_effect`が`true`のRouter manifestを拒否
- raw JSON・registry digest・秘密値を画面へ出さず、承認・実行権限を一切追加しない

## [1.0.0] - 2026-07-02

### Added
- 初回リリース: xops spool状態・RAG research記事数・local LLM worker一覧を表示する
  読み取り専用bubbletea TUI
- `--dump`フラグ(1回描画してプレーンテキスト出力、デバッグ用)
- ピクセルアートロゴ、「⚠️ Safety / Scope」セクション
