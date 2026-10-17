---
status: accepted
pr: nil
date: 2026-10-13
related: [ADR-0004, ADR-0006]
---

# ADR-0005: SSH 接続と認証情報

## 背景と課題

`pkg/shell/ssh.go` の現状は次のとおり。

- 認証は、鍵ファイルかパスワードのみ。ssh-agent には対応せず、パスフレーズ付きの鍵は読めない。
- `HostKeyCallback` は `ssh.InsecureIgnoreHostKey()`。
- `~/.ssh/config` を読まず、ProxyJump にも対応しない。
- 接続のタイムアウトを設定していない。
- ポートは yaml のタグが `int` で、`port:` と書いても読まれない ([0008](0008-git-operation-fixes-and-input-safety.md))。

isucon-tare の運用では、1台目の root で生成した鍵を、GitHub の deploy key (書き込み可) と、2台目以降への SSH の鍵の両方に使っている。開発者の GitHub 公開鍵は `init.sh` で各サーバの `authorized_keys` に配られ、root でログインできる。

実行場所がローカル PC になると、GitHub への認証と、サーバへの接続の両方を、PC から行う必要がある。

## 検討した選択肢

| 案 | 却下理由 |
|---|---|
| `InsecureIgnoreHostKey` のまま | 自律的に動く LLM では、中間者攻撃の検知がなくなる |
| yaml に `host_key` を持たせる | 初回の取得は結局 TOFU になる。ホスト鍵は環境の事実で、ブランチごとに異なる yaml に置くとマージの衝突を増やす |
| サーバ接続に各自の鍵を使う | 追跡はしやすいが、設定が増える。共有鍵で足りる |
| ssh-agent と `~/.ssh/config` に対応する | 現状の運用で不要。共有鍵はパスフレーズなしで作る |
| `password` を削除する | 後方互換を壊す |

## 決定

### 対象サーバへの接続

- Go の ssh クライアントで直接接続する。
- 鍵は、**共有の deploy key 1本**を使う (GitHub 用と同じ鍵)。パスは `{{.Env.X}}` で渡す ([0004](0004-host-configuration.md))。
- ssh-agent、パスフレーズ付き鍵、`~/.ssh/config`、踏み台 (ProxyJump) には対応しない。必要になった時点で再検討する。
- 接続のタイムアウトは **10秒** とする。

### ホスト鍵の検証

- **TOFU** (Trust On First Use) を採用する。初回の接続で受け入れて記録し、以降は検証する。
- 記録先は isucontinuous 専用の known_hosts ファイルとし、`~/.ssh/known_hosts` は使わない (ISUCON のサーバは作り直されて同じ IP でも鍵が変わりうるため)。
- 不一致のときは**エラーで止める**。MCP 経由では回避できず、解除は人間のみが行う。
- yaml に `host_key` を持たせることはしない。

### GitHub への認証

- repo 限定の deploy key (書き込み可) を、各開発者の PC に配る。
- `isucontinuous init` が、local-repo に `core.sshCommand` (`ssh -i <鍵> -o IdentitiesOnly=yes`) を設定し、deploy key だけを使わせる。開発者個人の GitHub 鍵は使わない。
- 鍵はコンテストごとに新規リポジトリへ登録するので、失効はその deploy key を削除するだけで済む。

### パスワード認証

- 使わない運用とする。フィールドは残し (後方互換)、`password` が非空のときは**警告**を出す (ホストごとに1回)。ただし yaml の値にテンプレート式 (環境変数経由) を含む場合は警告しない。
- yaml に平文の `password` があるまま `push` するときは、結果に警告を出す。

## 影響

- 同じ鍵が root の SSH にも使われるが、開発者は GitHub 公開鍵で既に root に入れるため、権限は増えない。
- PC に秘密鍵が置かれる。漏洩したら deploy key を削除し、鍵を再発行する。
- isucon-tare の prologue に、鍵の配布手順と環境変数の設定を追加する必要がある (別リポジトリ)。

## 未決事項

- [ ] `init` に渡す鍵パスの引数名
- [ ] 専用 known_hosts のパス
- [ ] ホスト鍵の不一致を人間が解除するためのコマンド名

## 実装時に決めたこと

(未実装)
