---
status: accepted
pr: "#37"
date: 2026-10-13
related: [ADR-0003, ADR-0005, ADR-0006]
---

# ADR-0004: ホスト設定

## 背景と課題

- 作業ブランチの `isucontinuous.yaml` に対象ホストだけを書く運用では、他ホストの設定をコメントアウトする手間があった。
- 実行場所がローカル PC になると、サーバ上で成立していた前提が崩れる。
  - `{{.Env.X}}` は実行プロセスの環境変数 (`os.Environ()`) で展開される (`pkg/template/template.go`)。tare では、これらを対象サーバの `/etc/isucon.bashrc` に置いていた。
  - `host` が `localhost` / `127.0.0.1` だと、SSH ではなくローカルのシェルで実行される (`Host.IsLocal`)。雛形は `host: localhost` で、PC で動かすと**開発者自身の PC に配置し、`post_command` を実行してしまう**。
  - `host` は接続先アドレス・local-repo のディレクトリ名・表示名を兼ねている。
- `deploy` は yaml を2回読む。1回目は checkout 前 (Slack クライアントの生成)、2回目は checkout 後 (ホスト一覧)。

## 検討した選択肢

| 案 | 却下理由 |
|---|---|
| `--host` 引数でホストを選択する | yaml をブランチごとに持つ運用と二重になる |
| コメントアウトで運用する | 手間が増える。差分も大きくなる |
| yaml の外の個人設定ファイルで接続情報を上書きする | 設定の置き場所が増える。環境変数のテンプレート展開で足りる |
| `localhost` の推定を維持する | PC で雛形のまま動かすと、自分の PC に配置してしまう |

## 決定

### ホスト選択

- 対象ホストは yaml の内容で決まる。作業ブランチの `isucontinuous.yaml` には、そのブランチで扱うホストだけを書く。
- yaml の読み込みは、checkout 後に統一する。Slack の設定もこの yaml から読む ([0002](0002-notification-and-artifact-limits.md))。

### `disabled`

- ホストエントリに `disabled: true` を指定できる。
- 設定の読み込み時に除外する。**SSH クライアントを作る前**に弾く。
- 除外したホストには、接続もロックもしない。リポジトリ内のホストディレクトリや、symlink の解決 (別ホストのディレクトリを辿る) には影響しない。
- すべてのホストが `disabled` の場合はエラーにする。
- `deploy.targets` の個別エントリには付けない (ホスト単位のみ)。
- `status` では「disabled」と表示し、`release` は `--include-disabled` で対象にする ([0003](0003-host-lock.md))。

### main への混入防止

- main (master) では常に全ホストを有効にし、`disabled` はマージ時に戻す。手順は skill に記載する ([0009](0009-skill-design.md))。
- 機械的にも、main (master) への `push` 時に `disabled` があれば警告する。

### `name` と `local`

- `name` を任意で追加する。local-repo のホストディレクトリ名とロックなどの表示名に使う。省略時は `host`。
- ローカル実行は、明示の `local: true` のときだけにする。`host` 文字列からの推定 (`localhost` / `127.0.0.1`) は廃止する。雛形は実アドレスの例にする。

### テンプレート展開

- `host` / `user` / `key` / `port` / `password` にも `{{.Env.X}}` の展開を効かせる。評価は設定の読み込み時に行い、使える変数は `.Env` のみ (`.Git.Revision` はエラー)。
- 設定の読み込み時に、**コマンドを含めたすべてのテンプレートを事前検証する**。環境変数の欠落は、ロックを取る前に失敗させる (現状は `deploy` の実行中、ロック取得後に失敗する)。
- ローカル PC では、必要な環境変数 (`ACCESSLOG_PATH`, `SLOWLOG_PATH` など) を MCP クライアントの設定などで渡す。isucon-tare の prologue にその手順を追加する。tare の `env/*` は `LOCAL_REPO=/root/local-repo` のようにサーバ固有の値を含むため、そのまま PC に持ち込めない。

## 影響

- `host: localhost` を使う既存の yaml は、`local: true` への移行が必要 (破壊的変更)。
- ブランチごとに `hosts` の内容が違うと、マージ時の衝突が増える。「main では全ホスト有効」の運用で対処する。
- `name` を使うと、ホストのディレクトリ名が IP に縛られず、サーバの作り直しで変わらない。

## 未決事項

- [x] `host: localhost` を使う既存の yaml への移行措置 (警告で案内する案など)
- [x] 雛形 (`skelton.yaml`) の更新内容

## 実装時に決めたこと

実装時に次のとおり決めた。

- `host: localhost` を使う既存の yaml の移行措置: `host` が `localhost` / `127.0.0.1` で `local` が未指定なら、設定の読み込みでエラーにし、`local: true` か実アドレスを促す。警告で続行すると、ローカル実行にも SSH 接続にもなり得て、PC で自分の PC にデプロイする余地が残るため。`local: false` の明示は未指定と区別しない。
- 雛形 (`skelton.yaml`): `host: 192.0.2.1` の実アドレス例にし、`name: isu1`、`local: false`、`disabled: false` を載せた。
- `disabled` のホストは、環境変数の検証を含め、一切評価しない。PC に存在しない環境変数を参照していても読み込める。
- 有効なホスト間で `name` (省略時は `host`) が重複したらエラーにする。ホストディレクトリ名と内部のキーが衝突するため。
- main (master) への `push` 時の `disabled` の検査は、テンプレートを評価しない。環境変数なしで動く。

