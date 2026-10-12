# ADR-0001: MCP サーバ化の全体設計

- 状態: Accepted (設計合意済み・未実装)
- 日付: 2026-10-13
- 関連: [0002](0002-notification-and-artifact-limits.md), [0003](0003-host-lock.md), [0009](0009-skill-design.md)

## 背景

現状の isucontinuous は人間が CLI で使うことを前提にしている。

- 進捗は `pb` のプログレスバー (`pkg/cmd/common.go` の `perHostExec`) で、TTY を前提にしている。
- ログは zap の Console 形式でファイルにのみ出力される。
- `afterbench` の結果は Slack とサーバ上のファイルに流れるだけで、呼び出し元に値として返らない。
- 1台目のサーバ上で root として実行する運用で、複数開発者の同時操作を調停する仕組みがない。

LLM が `deploy` → ベンチマーク → `profiling` → `afterbench` → 結果の読解、というループを自律的に回せるようにしたい。

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

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| `--output json` で CLI の出力を機械可読にする | MCP で公開するなら、進捗は progress notification、結果はツールの戻り値で足りる |
| `bench` / `experiment` サブコマンドで、ベンチの実行まで取り込む | ベンチはブラウザからの手動実行で、Playwright MCP で自動化できる。isucontinuous が持つ必要がない |
| スコアなどの実験記録を isucontinuous に持たせる | スコアは Playwright MCP など外部から得るデータ。skill 側に置くのが自然 |
| MCP を対象サーバ上で動かし、SSH 経由の stdio で起動する | 実行場所はローカル PC とする要件による |

## 影響

- 実行場所の変更により、設定の読み込み ([0004](0004-host-configuration.md))、SSH と認証 ([0005](0005-ssh-and-credentials.md))、local-repo の運用 ([0006](0006-local-repo-operation.md)) が影響を受ける。
- 従来 root で動かしていたため顕在化しなかった問題がある ([0008](0008-git-operation-fixes-and-input-safety.md) の C7)。

## 未決事項

- MCP のトランスポートの詳細 (ローカル PC 上でクライアントが起動する前提)
