# User Guide

## Preparation to use isucontinuous

* isucontinuous 管理対象サーバ上に事前に以下のコマンドがインストールされている必要があります。
    * git
    * curl

* isucontinuous 実行サーバにある秘密鍵に対応する公開鍵が isucontinuous 管理対象サーバ上に配置されている必要があります。

* isucontinuous 実行サーバにある秘密鍵に対応する公開鍵が、isucontinuous で管理するリポジトリに対応する GitHub Repository に登録されている必要があります。

## Setup Slack Bot

isucontinuous からのデプロイ通知やプロファイルデータを Slack に送信したい場合、Slack にて Bot User OAuth Token を取得する必要があります。

注意点として、Bot User は以下の Scope を持っている必要があります。

* `chat:write.public`
* `files:write`

