---
status: accepted
pr: nil
date: 2026-10-13
related: [ADR-0001, ADR-0005, ADR-0006]
---

# ADR-0008: 既存実装の修正と入力の安全性

## 背景と課題

MCP を前提としない既存の不具合と、LLM が入力する値を扱う際の安全性の問題を、まとめて扱う。

## 検討した選択肢

| 案 | 却下理由 |
|---|---|
| `Push()` の失敗を引き続きログだけにする | LLM が成功と誤認する |
| `safe.directory` をコマンドラインで指定する | 保護された設定の扱いが git のバージョンに依存し、古い Ubuntu の git では効かない可能性がある (未検証) |
| `git add -A` を、ファイルを指定した `git add` に変える | `import` の結果をまとめて取り込む現状の動作を変えてしまう |

## 決定

### C1: `Push()` のエラー伝播

- 現状 (`pkg/localrepo/localrepo.go`): `git commit` と `git push` の失敗を Info ログに書くだけで、`nil` を返す。認証失敗や reject でも成功と報告される。
- 変更なしのときの `git commit` は終了コード 1、すでに最新のときの `git push` は終了コード 0 を返す (git の挙動を確認)。
- 修正: `git add -A` の後に `git diff --cached --quiet` でコミット対象の有無を判定し、変更があるときだけコミットする。`commit` と `push` のエラーは呼び出し元へ伝播する。

### C2: commit message の指定

- `push -m` と MCP の `message` 引数を追加する。未指定時は現状の文言 (`commit by isucontinuous`) を使う。
- メッセージの渡し方は `git commit -F -` (標準入力) か引数配列とし、シェル文字列に埋め込まない。

### C3: `push` の結果

- コミットしたファイルの一覧 (サイズ付き) を結果に含める。
- 1 MiB 以上のファイルは警告する。ブロックはしない (リポジトリは使い捨てで、100 MB を超えれば GitHub 側が拒否し、C1 の修正でエラーとして届く)。

### C4: `ListUntrackedFiles`

- 現状 (`pkg/usecases/imports/imports.go`): 対象サーバのディレクトリで `git init` し、`git ls-files --others --exclude-standard` を実行し、`defer` で `rm -rf .git` する。
- 問題 (再現して確認):
  - 対象がすでに git リポジトリだと、`rm -rf .git` で**既存の履歴が消える**。
  - 対象が別のリポジトリのサブディレクトリだと、入れ子の `git init` が親の `.gitignore` を無視する。
- 修正: 対象が既存のワークツリー内なら、そのリポジトリを使う (`git rev-parse --show-toplevel`)。そうでなければ、一時ディレクトリを `--git-dir`、対象を `--work-tree` に指定する。いずれも対象に `.git` を作らず、削除もしない。
- `safe.directory` のグローバル設定への追記は、既存でなければ行う (重複して追記しない)。

### C5: `Host.Port` の yaml タグ

- 現状 (`pkg/config/types.go`): `yaml:"int,omitempty"` で、`port:` と書いても読まれない。
- 修正: `yaml:"port,omitempty"`。リポジトリ内で `int:` を使っている箇所はない。

### C7: `import` のファイルモード

- 現状: `stat -c '%a'` が返す8進数の文字列 (`644`) を、`strconv.Atoi` で10進数として読み、`os.FileMode` にしている。`644` が `--w----r--` (0204) になる。
- root で動かしていたため権限が無視されて顕在化しなかった。**一般ユーザーで動かすと、所有者に読み取り権限のないファイルができ、`git add -A` が `Permission denied` で失敗する** (確認済み)。
- 修正: 8進数として解析する (`strconv.ParseUint(s, 8, 32)`)。

### C8: `Deployer.Deploy` の走査コールバックが新しい Go で panic する

- 現状 (`pkg/usecases/deploy/deploy.go`): `filepath.WalkDir` のコールバックが `info != nil && !reflect.ValueOf(info).IsNil()` で `DirEntry` の nil を判定している。
- 問題: 新しい Go (Go 1.26.1 で確認) ではルートの `DirEntry` が構造体になり、`reflect: call of reflect.Value.IsNil on struct Value` で panic する。`go test ./pkg/usecases/deploy/` が main で失敗する。CI は Go 1.17 のため気づかれなかった。
- 修正: nil の判定を `reflect` に頼らず、コールバックの `err` 引数で行う。走査エラーの扱いは、既存のテスト (`testdata` の symlink エラーを含む) が期待する現状の挙動を保つ。エラーを無視している点の妥当性は、[0007](0007-drift-detection-and-deploy-plan.md) の計画と実行の分離で扱う。

### B4: シェル文字列の組み立て

- `revision` と `branch` は、`git checkout -d remotes/origin/%s` のように、検証もクォートもなしでシェル文字列に埋め込まれている。MCP 経由では LLM が値を渡す。
- 修正: `git check-ref-format` 相当で検証し、クォートする。

## 影響

- C7 の修正まで、一般ユーザーでの `import` → `push` は失敗する。ローカル PC 実行 ([0001](0001-mcp-server-architecture.md)) の前に直す必要がある。
- C3 の警告は ([0001](0001-mcp-server-architecture.md)) の警告の仕組みに載せる。

## 未決事項

- [ ] `safe.directory` を使わない方法の有無の調査

## 実装時に決めたこと

(未実装)
