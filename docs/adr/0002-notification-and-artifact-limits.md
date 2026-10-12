# ADR-0002: 通知と成果物の扱い

- 状態: Accepted (設計合意済み・未実装)
- 日付: 2026-10-13
- 関連: [0001](0001-mcp-server-architecture.md), [0004](0004-host-configuration.md), [0009](0009-skill-design.md)

## 背景

### Slack

- `cmd/deploy.go` と `cmd/afterbench.go` は、`slack-token` を全コマンドで必須にしている (`setRequired`)。
- 一方 `pkg/slack/slack.go` の `NewClient` は、`AuthTest` に失敗すると**黙って FakeClient に切り替える** (Info ログのみ)。空のトークンでも同じ。
- `deploy` は、デプロイ前の `SendText` が失敗すると中断する (`pkg/cmd/deploy.go`)。
- 本番では人間が Slack のメッセージを見て状況を把握するため、通知は必要。

### 成果物

- `afterbench` は、サーバ上のコマンドで alp・pt-query-digest・pprof の出力を作り、Slack に全文を投稿する。出力ディレクトリは終了時に `<target>.<unix時刻>` へ移動され、サーバ上に残る。
- MCP の応答が大きいと、クライアントによっては扱えない。Claude Code は既定で 25,000 トークンを超えると**エラーにする**と報告されており、その場合は応答が全部失われる。Codex は先頭と末尾を残して切り詰める。

## 決定

### Slack の有効判定

- `slack.default_channel_id` が非空なら Slack を**有効**、空なら**無効** (通知しない)。`setRequired("slack-token")` は廃止する。
- 有効なのにトークンが未設定、認証失敗、送信失敗の場合は、**警告を出して継続**する。`deploy` は中断しない。
- 警告は結果に含める ([0001](0001-mcp-server-architecture.md))。本番で通知が黙って消えることを防ぐ。
- 判定に使う yaml は、checkout 後の yaml に統一する ([0004](0004-host-configuration.md))。

### MCP 応答の上限

- ファイルごとに **8 KiB**、応答全体で **48 KiB** を上限とする。設定で変更できる。
- 切り詰めは行単位で、**先頭を残す** (alp・pt-query-digest・pprof はいずれも重要度順に出力されるため)。
- 切り詰めたら、応答に元のサイズとサーバ上のパスを含めて伝える。
- Slack には従来どおり全文を投稿する (切り詰めない)。

### 成果物を絞る方法

出力の絞り込みは `after_bench.command` に書く値 (alp の `-m`、pt-query-digest の `--limit`、pprof の `-nodecount` など) で行う。isucontinuous のコードでは行わず、書き方の知識は skill に置く ([0009](0009-skill-design.md))。

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| クライアントの自動切り詰めに依存する | Claude Code ではエラーになり、応答が全部失われる |
| 有効なのに認証失敗したらエラーにする | 通知の失敗でデプロイを止めたくない。警告で可視化すれば足りる |
| 要約パーサを isucontinuous に実装する | ツールの出力形式に依存し、保守が増える |
| ドリルダウン用ツール (`pprof -list` など) | 当面作らない |

## 影響

- 上限を超える設定ミス (alp の `-m` なしで URI が爆発するなど) があっても、応答は届き、切り詰めの旨が分かる。
- トークン換算は概算 (1トークン 2.5〜3 文字と見積もり、実測していない)。

## 未決事項

- 上限値の妥当性の確認
- ドリルダウン用ツールの要否

## 参考

- [Claude Code MCP tool exceeds maximum allowed tokens 25000 (Xpoz)](https://help.xpoz.ai/en/articles/12681842-claude-code-mcp-tool-exceeds-maximum-allowed-tokens-25000)
- [Codex CLI v0.152.0 Stable: Per-Tool MCP Output Limits](https://codex.danielvaughan.com/2026/09/01/codex-cli-v0152-stable-per-tool-mcp-output-limits-app-server-timeouts-compaction-fixes/)
