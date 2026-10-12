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

注意点として、Bot User は以下の Scope を持っている必要があります。

* `chat:write.public`
* `files:write`
* `channels:read`

## MCP サーバとして使う

`isucontinuous mcp` は、CLI のサブコマンド (`init`, `setup`, `import`, `push`, `sync`, `deploy`, `profiling`, `afterbench`, `release`, `status`, `diff`) を MCP のツールとして stdio で公開します。MCP クライアントが各開発者のローカル PC 上でプロセスを起動する前提で、対象サーバには SSH で接続します。`forget` と `sync` の `--force` は人間のみが使うため、公開しません。

Claude Code に登録する例:

```
claude mcp add isucontinuous \
  -e LOCAL_REPO=$HOME/local-repo \
  -e SLACK_TOKEN=xoxb-... \
  -- isucontinuous mcp
```

* 設定は CLI と同じフラグ・環境変数 (`--local-repo` / `LOCAL_REPO`、`--logfile` / `LOGFILE`、`--log-level` / `LOG_LEVEL`、`--known-hosts` / `KNOWN_HOSTS`) で渡します。`isucontinuous.yaml` が参照する環境変数も、MCP クライアントの `-e` で渡してください。
* Slack の通知先は環境変数 `SLACK_TOKEN` から読みます。ツールの引数には取りません。
* ログは標準出力には出さず、`--logfile` のファイルに書きます (標準出力は MCP のプロトコルが使うため)。
* ロックの所有者は CLI と同じ `user@host` です。CLI で取ったロックを MCP から続けて使えます。
* 長く掛かるツールは、クライアントが progress token を渡すと、ホストとタスクごとに progress notification を送ります。
