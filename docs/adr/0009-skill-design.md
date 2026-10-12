# ADR-0009: skill 設計

- 状態: Proposed (骨子のみ。網羅的な内容は後日検討する)
- 日付: 2026-10-13
- 関連: [0001](0001-mcp-server-architecture.md), [0002](0002-notification-and-artifact-limits.md), [0003](0003-host-lock.md), [0004](0004-host-configuration.md), [0006](0006-local-repo-operation.md)

## 背景

isucontinuous を MCP で公開しても、LLM が ISUCON を進めるには、ツールの使い方以外の知識が要る。そのうち次のものは、isucontinuous のコードではなく skill に置くのが適切である。

- Playwright MCP など、isucontinuous の外側でしか得られないデータの扱い (ベンチマークの実行、スコア)
- `isucontinuous.yaml` に書く値の知識 (alp、pt-query-digest、pprof のオプション)
- 運用上の約束事 (ブランチ運用、`disabled` の戻し方)

## 決定

### 役割分担

| 置き場所 | 内容 |
|---|---|
| MCP のツール説明 | 守らないと事故になる規則。skill は発動したときにしか読み込まれないため、ツール側にも書く |
| skill | 手順と知識 |
| `isucontinuous.yaml` / 環境変数 | 大会ごとの差分 (パス、コマンド) |

### ツール説明に書く規則

- `--force` (ロックの強制取得) は、保持者に確認してから使う ([0003](0003-host-lock.md))
- `--ignore-drift` は、サーバ上の直接編集を上書きしてよいと確認してから使う ([0007](0007-drift-detection-and-deploy-plan.md))
- ホスト鍵の不一致は、人間が確認するまで回避しない ([0005](0005-ssh-and-credentials.md))
- 警告は読んで対処する ([0001](0001-mcp-server-architecture.md))

### skill に書く内容 (現時点で決まっているもの)

1. **全体の流れ**
   - 初期セットアップ: `init` → yaml の編集 → `setup` → `import` → `push`
   - 開発ループ: 別クローンで編集 → commit → push → `deploy` → (Playwright MCP でベンチ) → `profiling` → `afterbench` → 結果の読解 → 記録 → 次の仮説
   - 新しいファイルの管理: yaml に追記 → `sync` → `import` → `push`
2. **ロックの扱い** ([0003](0003-host-lock.md))
   - `deploy` で取得し、`afterbench` の成功で解放される
   - `afterbench` が失敗したらロックは保持される。原因を直して再実行するか、`release` で解放する
   - 他人が保持している場合は、保持者を報告し、`--force` は人間の確認後に使う
3. **`disabled` の約束事** ([0004](0004-host-configuration.md)): main では常に全ホスト有効にし、`disabled` はマージ時に戻す
4. **ベンチマークの実行とスコアの取得**: Playwright MCP を使う手順 (具体は大会ごと)
5. **実験記録**: 置き場所と形式を skill で決める。記録する項目は、revision (ハッシュ)、変更内容、スコア、プロファイルの要約、仮説
6. **プロファイル成果物の読み方と、yaml コマンドの書き方** ([0002](0002-notification-and-artifact-limits.md))
   - alp: `-m` (URI のグルーピング。LLM が育てる成果物)、`--sort sum`
   - pt-query-digest: `--limit`、`--report-format`
   - pprof: `-top -cum -nodecount`、fgprof
7. **push してから deploy する** ([0006](0006-local-repo-operation.md)): 未 push の変更はデプロイされない。local-repo は手で編集しない。WIP ブランチの運用
8. **警告の読み方**: Slack 無効、`password` の平文、`disabled` の混入など
9. **環境構築**: isucon-tare の prologue、環境変数、鍵の配布への参照 ([0005](0005-ssh-and-credentials.md))

### 構成案

```
SKILL.md                 概要、全体の流れ、ツール一覧
references/
  locking.md             ロックの扱い
  profiling.md           成果物の読み方、yaml コマンド
  recording.md           実験記録
  setup.md               環境構築への参照
```

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| すべての規則を skill に書く | skill は発動したときにしか読み込まれず、守らないと事故になる規則が漏れる |
| 成果物の絞り込みをコードで行う | オプションは yaml の値で、実装ではない ([0002](0002-notification-and-artifact-limits.md)) |
| 大会ごとの差分を skill に入れる | yaml・環境変数・prologue の役割 |

## 未決事項

- skill の配置先 (本リポジトリか、別の場所か)
- 実験記録の置き場所と形式
- 網羅的な内容の検討 (後日)
