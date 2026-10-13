---
status: accepted
pr: "#36"
date: 2026-10-13
related: [ADR-0002, ADR-0003, ADR-0009]
---

# ADR-0001: MCP サーバ化の全体設計

## 背景と課題

現状の isucontinuous は人間が CLI で使うことを前提にしている。

- 進捗は `pb` のプログレスバー (`pkg/cmd/common.go` の `perHostExec`) で、TTY を前提にしている。
- ログは zap の Console 形式でファイルにのみ出力される。
- `afterbench` の結果は Slack とサーバ上のファイルに流れるだけで、呼び出し元に値として返らない。
- 1台目のサーバ上で root として実行する運用で、複数開発者の同時操作を調停する仕組みがない。

LLM が `deploy` → ベンチマーク → `profiling` → `afterbench` → 結果の読解、というループを自律的に回せるようにしたい。

## 検討した選択肢

| 案 | 却下理由 |
|---|---|
| `--output json` で CLI の出力を機械可読にする | MCP で公開するなら、進捗は progress notification、結果はツールの戻り値で足りる |
| `bench` / `experiment` サブコマンドで、ベンチの実行まで取り込む | ベンチはブラウザからの手動実行で、Playwright MCP で自動化できる。isucontinuous が持つ必要がない |
| スコアなどの実験記録を isucontinuous に持たせる | スコアは Playwright MCP など外部から得るデータ。skill 側に置くのが自然 |
| MCP を対象サーバ上で動かし、SSH 経由の stdio で起動する | 実行場所はローカル PC とする要件による |

## 決定

### 1. MCP サーバとして公開する

- isucontinuous を MCP サーバとして公開する。MCP プロセスは**各開発者のローカル PC** で動作し、対象サーバには SSH で接続する。
- 対象サーバ上では動かさない。

### 2. 責務の境界

| 持つ | 持たない | 担当 |
|---|---|---|
| ファイルの同期・デプロイ、プロファイル取得、ホストのロック | ベンチマークの実行 | Playwright MCP |
| | 実験記録 (スコアなど) | skill ([0009](0009-skill-design.md)) |
| | 結果の解釈・仮説立案 | LLM |
| Slack などへの通知 (出力先の1つとして) | | |

### 3. 構造

- usecase 層は**型付きの結果**を返す。CLI は整形したテキスト、MCP は構造化データとして出力する。
- 進捗の通知先は **Reporter インタフェース**で差し替える。CLI は `pb`、MCP は progress notification。
- `--output json` のような JSON 出力モードは作らない。
- エラーは種別・ホスト・タスクを持つ型で返す (`pkg/errors` の型を活用する)。
- **警告**は結果の構造体に含める。CLI では stderr、MCP では応答に載せる。ログファイルへの出力だけにしない。

### 4. CLI と MCP の整合

- MCP のツール名と引数は CLI のサブコマンドと 1:1 にし、意味も揃える。
- 同じ usecase 層を呼ぶ薄いアダプタにする。
- ロックの所有者は `user@host` で識別する。CLI で取ったロックを MCP から続けて使え、逆も可能にする。
- `--force` の意味は、どちらの経路でも同じにする。

### 5. 公開するツール

既存のサブコマンド (`init`, `setup`, `import`, `push`, `sync`, `deploy`, `profiling`, `afterbench`) に加え、次を新設する。

| 新規 | 内容 | ADR |
|---|---|---|
| `release` | ロックの明示的な解放 | [0003](0003-host-lock.md) |
| `status` | 保持者と最後にデプロイした revision の一覧 | [0003](0003-host-lock.md) |
| `diff` | サーバ上のファイルと revision の比較 | [0007](0007-drift-detection-and-deploy-plan.md) |

## 影響

- 実行場所の変更により、設定の読み込み ([0004](0004-host-configuration.md))、SSH と認証 ([0005](0005-ssh-and-credentials.md))、local-repo の運用 ([0006](0006-local-repo-operation.md)) が影響を受ける。
- 従来 root で動かしていたため顕在化しなかった問題がある ([0008](0008-git-operation-fixes-and-input-safety.md) の C7)。

## 未決事項

- [x] MCP のトランスポートの詳細 (ローカル PC 上でクライアントが起動する前提。ADR-0001-B で決定)

## 実装時に決めたこと

### ADR-0001-A: 土台

実装済み: Reporter インタフェース (`ProgressReporter` / `RecordingReporter`)、各 `Run*` の型付き結果と `Warnings`、`pkg/errors.HostTask`。`mcp` サブコマンド、ツール公開、progress 通知は未実装。

決めた未決事項:

- `HostTask.Error()` は内側のメッセージをそのまま返す。ホスト・タスクを前置すると CLI のエラー出力が変わるため。文脈は `errors.As` で取り出す。
- deploy の `hash:` / `message:` は結果に移したため、CLI ではプログレスバーの後に表示される (失敗時も取得済みなら表示)。
- push の警告は stdout から stderr に変わった。afterbench の cleanup 失敗は、ログに加えて警告として stderr に出る。
- 失敗時も取得済みの結果 (ホストごとの完了タスク、deploy の hash) は非 nil で返す。

### ADR-0001-B: MCP サーバ本体

実装済み: `isucontinuous mcp` (`cmd/mcp.go`、実装は `pkg/mcpserver`)、11 ツールの公開、progress 通知。

決めた未決事項:

- トランスポートは stdio のみ。公式 SDK `github.com/modelcontextprotocol/go-sdk` の `v1.3.1` を使う (Go 1.23.0 が最小の v1 系)。go.mod の `go` を `1.23.0` に上げた。
- ログは標準出力に出さず、既存の `--logfile` のファイルにのみ書く。`mcp` では「output log to」の表示も出さない。
- Slack トークンは引数にせず、環境変数 `SLACK_TOKEN` から読む。LLM の入力に載せないため。
- 公開しない: `forget`、`sync` の `force`。`setup` と `import` には、CLI と同じく `force` を付けない ([0003](0003-host-lock.md))。
- 結果は `pkg/cmd` の型付き構造体 (`Warnings` を含む) をそのまま返す。エラーは `isError` の結果にし、本文に `kind` (Go の型名)・`host`・`task`・`message` を JSON で載せる。途中までの結果も構造化データとして返す。
- progress は、クライアントが progress token を渡したときだけ通知する。タスクの開始と完了を 1 件ずつ数え、total は ホスト数×タスク数×2 とする。token がないときは何も出さない (既定の `pb` は標準出力に描画し、stdio を壊すため)。
- ロックの所有者は `ConfigCommon.Owner` の既定 (`user@host`) のままにする。
- `v1.3.1` の `CallToolParams.SetProgressToken` は `Meta` が nil だと token を保存しない。サーバ側の処理には影響しないため、テストでは `Meta` を直接設定する。

