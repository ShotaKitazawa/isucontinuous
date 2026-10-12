# ADR-0006: local-repo の運用

- 状態: Accepted (設計合意済み・未実装)
- 日付: 2026-10-13
- 関連: [0003](0003-host-lock.md), [0004](0004-host-configuration.md), [0005](0005-ssh-and-credentials.md), [0007](0007-drift-detection-and-deploy-plan.md)

## 背景

local-repo は、isucontinuous を実行するマシン上にある git の clone である。従来は1台目のサーバ、今回の設計では各開発者の PC に、それぞれ存在することになる。

現状の挙動は次のとおり。

- `init` は `git init` と `remote add` を行い、雛形の `isucontinuous.yaml` と `.gitignore` を作る。既存のリモートを取り込む手段がない。
- `import` は作業ツリーに書き、`push` が `git add -A` でコミットして master に送る。
- `deploy` は fetch して revision を detached HEAD で checkout し、`filepath.WalkDir` で**ファイルシステムを走査**して配置する。
- `sync` は、detached なら `git reset --hard` してから fetch し、ブランチに切り替える。
- 2人目以降が `init` してから `sync` する手順は、`git checkout master` が「untracked working tree files would be overwritten」で失敗する (git の挙動を再現して確認)。

## 決定

### 位置づけ

- local-repo は **isucontinuous 専用** (import と deploy) とする。手で編集しない。
- コーディングは**別のクローン**で行い、デプロイの前に commit と push をする。
- 1回の実験ごとに commit と push が必要になるが、受容する。WIP ブランチの運用は skill に書く ([0009](0009-skill-design.md))。

### ブートストラップ

- `init` は、リモートに既存のブランチがあれば、雛形を作らずに取り込む。
- 初期の `.gitignore` は作らない (`.revision` は廃止される。[0003](0003-host-lock.md))。
- ホストのディレクトリ名は `name` を使う ([0004](0004-host-configuration.md))。

### import と push の担当

- ツール側のガードは設けない。運用で1人に決め、skill に書く。`import` はホストロックを取るため同時実行は排他され、`push` の衝突は差分エラーで検知される。

### 破壊的操作の保護

- `sync` は、未コミットの変更があれば拒否する。`--force` で許可する。

### デプロイの整合性

| 課題 | 対応 |
|---|---|
| 未 push の変更が黙ってデプロイされない (origin の revision が使われる) | 結果に、解決したコミットハッシュ、メッセージ、日時を必ず含める |
| revision がブランチ名だと、push し直すと同じ名前で中身が変わる | ロックと `{{.Git.Revision}}` には、**解決したコミットハッシュ**を使う |
| 未追跡ファイルが、revision にないのにデプロイされる (`WalkDir` はファイルシステムを走査する) | デプロイ前に作業ツリーがクリーンでなければ**エラー**にする (自動で消さない) |
| `sync` / `import` / `push` で checkout が変わると、`profiling` / `afterbench` が別の yaml・別の revision で動く | ロックの revision と HEAD が一致しなければ**エラー**にする |
| 同じ PC で CLI と MCP、または複数セッションが同時に local-repo を操作する | local-repo 用の**ローカルロック**を置く。同一マシン上なので flock が適する |

## 検討した代替案

| 案 | 却下理由 |
|---|---|
| local-repo を開発用クローンと共用する | `deploy` の detached checkout が作業ツリーを壊す |
| デプロイ前に未追跡ファイルを自動で消す | `import` の未コミットの結果を消してしまう |
| 担当をツールで強制する | 3人の小さなチームでは、運用で足りる |

## 影響

- 開発者は、local-repo と開発用クローンの2つを持つ。
- ロックの revision と HEAD の一致チェックは、[0003](0003-host-lock.md) のロックに依存する。

## 未決事項

- ローカルロックのファイルの位置
- 「クリーン」の判定の詳細 (無視されたファイルを含めるか)
