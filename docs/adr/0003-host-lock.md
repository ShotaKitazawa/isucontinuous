---
status: accepted
pr: "#40"
date: 2026-10-13
related: [ADR-0001, ADR-0004, ADR-0006, ADR-0007]
---

# ADR-0003: ホストロック

## 背景と課題

複数の開発者 (と、それぞれの LLM エージェント) が、同じ対象サーバを同時に操作しうる。計測の最中に別の人が deploy すると、結果が壊れる。

現状は local-repo の `.revision` ファイルが、単一の「デプロイ済み」状態を持つ。`deploy` が書き、`afterbench` が成功時に消し、`deploy` は `.revision` があり revision が異なると「`afterbench` か `deploy --force` を実行してください」とエラーにする。ただしこれは local-repo ごと (= 開発者の PC ごと) の状態で、ホスト単位でも共有でもない。

## 検討した選択肢

| 案 | 却下理由 |
|---|---|
| SSH 先で `flock` を保持し続ける | プロセスや接続が切れるとロックを**黙って失う**。明示的なファイルならその問題がない |
| 中央に共有の MCP サーバを置く | ホスティングと認証が必要。ホスト単位の独立した操作には過剰 |
| GitHub の ref をロックに使う | サーバ側の状態と別の場所に真実が分散する |
| timeout や heartbeat | `--force` と会話で解消する方針。仕組みが増える |
| `--force` で保持者のプロセスを kill する | 厳密性は不要 |
| ホストを区別せず1つのロック | 開発者が別々のホストを同時に触れなくなる |

## 決定

### 方式

- 各対象サーバ上に**ロックファイルを明示的に作る**。作成はアトミックに行う (`mkdir` や noclobber)。
- ロックはホスト単位。複数ホストを対象にするときは**全部か無か**で、一部だけ取れたら取得済みを戻して失敗を報告する。待たずに失敗する (try-lock)。
- ロックの内容は、所有者 (`user@host`)、revision (解決済みのコミットハッシュ)、取得時刻、操作。
- 取得に失敗したときは、保持者の情報を応答に含める。
- timeout は設けない。残ったロックは `--force` で解消する。

### 取得と解放

| コマンド | ロックの扱い |
|---|---|
| `deploy` | 取得する。同じ所有者は再取得できる (再入可) |
| `setup` | コマンド内で取得して解放する (インストールが計測に影響するため) |
| `import` | コマンド内で取得して解放する (他人の実験中の状態を取り込まないため) |
| `profiling` | 自分のロックがあることを確認する |
| `afterbench` | 自分のロックがあることを確認する。**成功したホストのロックを暗黙に解放する** |
| `release` | 明示的に解放する |
| `diff` / `status` / 成果物の取得 / `sync` / `push` | ロックなし (読み取りのみ、またはサーバに触れない) |

- `deploy` が失敗したホストは、ロックを解放する。
- `afterbench` が失敗したホストは、**ロックを保持する**。アクセスログとスローログは次の deploy の `pre_command` で消えるため、保持しないと再実行ができなくなる。
- `profiling` と `afterbench` は、`{{.Git.Revision}}` をロックの revision から得る。`.revision` は廃止する。

### `--force`

- 厳密にはしない。既存のロックを削除または上書きして取得する (PID を kill するなどはしない)。開発者同士は会話で調整できる前提。
- MCP にも公開する。取得に失敗したときは保持者を返し、ツールの説明で「保持者に確認してから使う」ことを促す。

### `release` と `status`

- `release` は、既定で自分のロックを、有効なホストについて解放する。`--force` で他人のロックも解放する。`--include-disabled` で `disabled` のホスト ([0004](0004-host-configuration.md)) も対象にする。
- `status` は、ホストごとの保持者と、最後にデプロイした revision を一覧する。`disabled` のホストは「disabled」と表示する。

### R/W ロックは採用しない

ロックが必要なのは、サーバの状態を変更するコマンドと、サーバが安定した状態であることを前提にする `import` だけ。読み取りだけを許す候補は `import` のみで、短くまれにしか実行しないため、排他で足りる。

## 影響

- SIGKILL、PC のクラッシュ、ネットワーク断でロックが残りうる。解消は `release --force` で行う。
- 同じ開発者の CLI と MCP は、同じ所有者として扱われる。
- ロックとは別に、**local-repo 側のローカルロック**が必要 ([0006](0006-local-repo-operation.md))。

## 未決事項

- [x] ロックファイルを置くホスト上のパス
- [x] アトミック作成の具体的な方法 (`mkdir` か noclobber か)
- [x] MCP での `--force` の引数名
- [x] `status` の出力形式

## 実装時に決めたこと

- ロックを置くホスト上のパス: 「最後にデプロイした revision」と同じディレクトリ (`/var/lib/isucontinuous/`、書き込めなければ `/tmp/isucontinuous/`。定数は `pkg/hoststate`) の `lock/`。内容は `lock/info`。取得の前に両方のディレクトリを読むので、置き場所が食い違っても二重に取得されない。
- アトミック作成の方法: `mkdir`。noclobber (`set -C`) は shell の実装ごとに挙動の確認が要るが、`mkdir` は POSIX で原子的で、既存なら必ず失敗する。`mkdir` の後に `info` を書き、書けなければロックを削除して失敗する。`info` が空または無いロックは、保持者不明のロックとして扱う。
- `info` の形式: `owner=` / `revision=` / `acquired_at=` (RFC3339, UTC) / `operation=` の行。所有者は `user@host` (ローカル PC のユーザー名とホスト名)。
- 再入: 同じ所有者の再取得は成功する。内容を書き換えるのは `deploy` だけ (全ホストの取得に成功した後)。`setup` と `import` は内容に触れず、新しく作ったロックだけを解放する。自分の `deploy` のロックを消さないため。
- 全部か無か: 失敗したときは、その呼び出しで新しく作った (または奪った) ロックだけを解放する。再入だったホストのロックは残る。
- `--force`: `deploy` に付ける (他人のロックを削除して取得する)。以前の `.revision` のガードを外す `deploy --force` は廃止した。`setup` と `import` には付けず、`release --force` で解消する。
- `deploy` の drift 事前検査: ロック取得後に、最後にデプロイした revision との差を調べる。差があればエラーにして、取得したロックを解放する。`--ignore-drift` で省略する。記録のないホストは検査しない。
- `deploy` が失敗したホストのロックは解放する。再入で取っていた場合も解放する (内容を更新済みのため)。解放にはキャンセル済みでない context を使う。
- `{{.Git.Revision}}`: `deploy` を含むすべてのコマンドで解決済みのコミットハッシュにした (以前の `deploy` はユーザー指定の名前だった)。`profiling` と `afterbench` はホストごとにロックの revision から Templator を作る。`.revision`、`GetRevision` / `SetRevision` / `ClearRevision`、`.gitignore` への書き込みは廃止した。
- `setup` と `import` のロックの revision: local-repo の HEAD のコミットハッシュ (取れなければ空)。
- `release`: 既定は自分のロックだけ。他人のロックは「held by another owner」として報告し、エラーにはしない。`--force` で他人のロックも解放する。`--include-disabled` は `Config.Disabled` (設定の読み込みが除外前に保持する) の接続情報を使う。環境変数が未設定などで解決できないホストはエラーにする。
- `status` の出力形式: ホストごとに1行 (`host: locked by <owner> (operation, revision, 時刻) | unlocked; last deployed revision: <hash>`)。`disabled` のホストは `disabled`。結果は `StatusResult` (`HostStatus` のリスト)。`release` の結果は `ReleaseResult` (`HostRelease` のリスト)。
- MCP での `--force` の引数名: `force` (CLI のフラグと同じ)。MCP は未実装なので、ツールの説明で「保持者に確認してから使う」ことを促すのは MCP の実装で行う。

