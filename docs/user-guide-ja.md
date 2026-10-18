# User Guide

## Preparation to use isucontinuous

* isucontinuous 管理対象サーバ上に事前に以下のコマンドがインストールされている必要があります。
    * git
    * curl
    * unzip

* isucontinuous 実行サーバにある秘密鍵に対応する公開鍵が isucontinuous 管理対象サーバ上に配置されている必要があります。

* isucontinuous 実行サーバにある秘密鍵に対応する公開鍵が、isucontinuous で管理するリポジトリに対応する GitHub Repository に登録されている必要があります。

## Setup Slack Bot

isucontinuous からのデプロイ通知やプロファイルデータを Slack に送信したい場合、Slack にて Bot User OAuth Token を取得する必要があります。

Slack を使うかどうかは `isucontinuous.yaml` の `slack.default_channel_id` で決まります。

* 非空なら Slack を**有効**として扱い、`deploy` と `afterbench` が通知を送ります。トークンは環境変数 `SLACK_TOKEN` (または `--slack-token`) で渡します。
* 空なら Slack を**無効**として扱い、通知も警告も出しません。トークンも不要です。
* 有効なのに、トークンが未設定、認証に失敗、送信に失敗した場合は、**警告を出して処理を続けます**。デプロイは中断しません。警告は CLI では標準エラーに、MCP では応答の `Warnings` に出ます。通知が届いていないときは、まずここを確認してください。

注意点として、Bot User は以下の Scope を持っている必要があります。

* `chat:write.public`
* `files:write`
* `channels:read`

## MCP サーバとして使う

`isucontinuous mcp` は、開発ループで使うCLIサブコマンド (`setup`, `deploy`, `diff`, `status`, `profiling`, `afterbench`, `release`) をMCPのツールとしてstdioで公開します。MCPクライアントが各開発者のローカルPC上でプロセスを起動する前提で、対象サーバにはSSHで接続します。repositoryの初期化・取り込みに使う`init`と`import`は、実行場所と状態を人間が確認してCLIから実行します。同期とpushは`import`の内部フェーズであり、独立したCLI/MCP toolとしては公開しません。

Claude Code に登録する例:

```
claude mcp add isucontinuous \
  -e LOCAL_REPO=$HOME/local-repo \
  -e SLACK_TOKEN=xoxb-... \
  -- isucontinuous mcp
```

* 設定は CLI と同じフラグ・環境変数 (`--local-repo` / `LOCAL_REPO`、`--logfile` / `LOGFILE`、`--log-level` / `LOG_LEVEL`、`--known-hosts` / `KNOWN_HOSTS`) で渡します。`isucontinuous.yaml` が参照する環境変数も、MCP クライアントの `-e` で渡してください。
* Slack の通知先は環境変数 `SLACK_TOKEN` から読みます。ツールの引数には取りません。
* `afterbench` の応答には、成果物 (alp・pt-query-digest・pprof の出力) がホスト・サーバ上のパス・サイズ・内容として含まれます。クライアントが大きな応答を扱えないことがあるため、内容はファイルごとに 8 KiB、応答全体で 48 KiB までに切り詰めます。行の途中では切らず、先頭から残します。切り詰めた成果物には `Note` に元のサイズ、残したサイズ、サーバ上のパスが入ります。Slack には全文を投稿します。
    * 上限は `--max-file-bytes` / `MCP_MAX_FILE_BYTES` (ファイルごと、既定 8192) と `--max-response-bytes` / `MCP_MAX_RESPONSE_BYTES` (応答全体、既定 49152) で変えられます。単位はバイトです。フラグを環境変数より優先します。
    * 出力を絞るには、`after_bench.command` の alp の `-m`、pt-query-digest の `--limit`、pprof の `-nodecount` などを使ってください。
* ログは標準出力には出さず、`--logfile` のファイルに書きます (標準出力は MCP のプロトコルが使うため)。
* ロックの所有者は CLI と同じ `user@host` です。CLI で取ったロックを MCP から続けて使えます。
* 長く掛かるツールは、クライアントが progress token を渡すと、ホストとタスクごとに progress notification を送ります。
