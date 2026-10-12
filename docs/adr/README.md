# Architecture Decision Records

isucontinuous を LLM から利用できる形 (MCP サーバ化) にするための設計メモ。日本語で記述する。
**いずれも設計合意のみで、実装は未着手。**

## 一覧

| ADR | タイトル | 目的 |
|---|---|---|
| [0001](0001-mcp-server-architecture.md) | MCP サーバ化の全体設計 | 責務の境界、実行場所、CLI と MCP の整合 |
| [0002](0002-notification-and-artifact-limits.md) | 通知と成果物の扱い | Slack の有効判定、警告、MCP 応答のサイズ上限 |
| [0003](0003-host-lock.md) | ホストロック | 複数開発者・複数エージェントの同時操作の排他 |
| [0004](0004-host-configuration.md) | ホスト設定 | ホスト選択、`disabled`、`name`、`local`、テンプレート展開 |
| [0005](0005-ssh-and-credentials.md) | SSH 接続と認証情報 | ホスト鍵検証、鍵の扱い、deploy key、パスワード |
| [0006](0006-local-repo-operation.md) | local-repo の運用 | 実行マシンに置く clone の位置づけとデプロイの整合性 |
| [0007](0007-drift-detection-and-deploy-plan.md) | drift 検出とデプロイ計画 | サーバ側の直接編集の検知、転送の効率化 |
| [0008](0008-git-operation-fixes-and-input-safety.md) | 既存実装の修正と入力の安全性 | MCP 前提とは独立した不具合修正 |
| [0009](0009-skill-design.md) | skill 設計 | LLM 向けの手順と知識の置き場所 |

## 依存関係と実装順序の案

```
0008 (独立した修正)
  │
  ├─ 0004 (設定の読み込み) ──┬─ 0003 (ロック) ──┬─ 0006 (local-repo 運用)
  │                          │                  │
  ├─ 0005 (SSH・認証)        └─ 0007 (drift) ───┘
  │
  └─ 0001 (Reporter・型付き結果) ── 0002 (通知・上限) ── MCP サーバ本体 ── 0009 (skill)
```

1. **0008**: MCP を前提としない修正。先に着手できる。
2. **0004 / 0005 / 0001 の土台部分**: 設定の読み込み (テンプレート展開、`disabled`、`name`、`local`)、SSH の接続、Reporter と型付き結果。
3. **0007 → 0003 → 0006**: デプロイの計画と実行の分離を先に行い、ロックはその上に載せる。local-repo の整合性チェックはロックの revision に依存する。
4. **0002 と MCP サーバ本体**: 土台が揃ってから、CLI のサブコマンドと 1:1 でツールを公開する。
5. **0009**: ツールの仕様が固まってから確定する。下書きは先行して進められる。

## 横断的な未決事項

- ロックファイルと「最後にデプロイした revision」を置くホスト上のパス ([0003](0003-host-lock.md), [0007](0007-drift-detection-and-deploy-plan.md))
- `host: localhost` を使う既存の yaml の移行措置 ([0004](0004-host-configuration.md))
- フラグや引数の名前 (`--ignore-drift`、ホスト鍵の解除など)
- skill の配置先と網羅的な内容 ([0009](0009-skill-design.md))

## スコープ外

- isucon-tare リポジトリの `env/*` に Slack token が平文で含まれる件
- isucon-tare の prologue の更新 (別リポジトリ。必要な変更点は [0004](0004-host-configuration.md) と [0005](0005-ssh-and-credentials.md) に記載)

## 用語

- **実行マシン**: isucontinuous を動かすマシン。従来は1台目のサーバ、今回の設計では各開発者のローカル PC
- **local-repo**: 実行マシン上にある、isucontinuous が管理する git の clone (既定 `~/local-repo`)
- **対象サーバ**: SSH でファイルを配置・取得し、コマンドを実行する先のサーバ
