# Concept

isucontinuous は複数台のサーバを用いた複数人での開発を支援するツールです。
isucontinuous は複数台のサーバ上にあるソースコードを取り込み、継続的なデプロイメント及びプロファイリングを実現します。

isucontinuous には以下のそれぞれのフェーズを意識したコマンド体系が用意されています。

* `init`: Git ローカルリポジトリを初期化する

<img src="./images/init.jpg?raw=true" width=533 height=300>

* `setup`: 開発に必要なソフトウェアをインストールする

<img src="./images/setup.jpg?raw=true" width=533 height=300>

* `import`: サーバ上の任意のファイルを取得し、Git ローカルリポジトリへcommit・pushする

<img src="./images/import.jpg?raw=true" width=533 height=300>

* `push`: importのpush失敗時などに、Git ローカルリポジトリのcommitを再度pushする

<img src="./images/push.jpg?raw=true" width=533 height=300>

* `sync`: importを完了できない作業ツリーの復旧時などに、Git リモートリポジトリの内容を反映する

<img src="./images/sync.jpg?raw=true" width=533 height=300>

* `deploy`: 任意のリビジョンにおける Git リモートリポジトリ管理下のファイルをサーバ上にデプロイする

<img src="./images/deploy.jpg?raw=true" width=533 height=300>

* `profile`: 各サーバにて任意のコマンドを実行する

<img src="./images/profiling.jpg?raw=true" width=533 height=300>

* `afterbench`: 各サーバ上にて任意のコマンドを実行しプロファイルデータを生成後、指定したディレクトリ以下のファイルを Slack に POST する

<img src="./images/afterbench.jpg?raw=true" width=533 height=300>

## Usecase

### 1. 初期セットアップ

初期取り込みは1台目の対象サーバ上で実行します。2、3台目のファイルは1台目からLAN内のSSHで取得するため、全コンテンツを開発者PCへWAN転送しません。

1. `init` : 1台目にimport用ローカルリポジトリを新規作成
1. isucontinuous.yaml の編集
1. `setup` : 各種開発用ソフトウェアのインストール
1. `import` : 各種設定ファイルやソースコードを取得し、最初のcommitをGitHubへpush

### 2. 開発・デプロイ

以降は各開発者のローカルPCからCLIまたはMCPを実行します。

1. GitHub に push したリポジトリを元に各メンバーが各ブランチで作業
1. `deploy` : 特定リビジョンの各ファイルを各サーバにデプロイ
1. `profiling` : 各サーバにてプロファイリング用コマンドを実行
1. `afterbench` : プロファイルデータを収集し Slack に送信

### 3. 開発中サーバ上の新たなファイルを Git で管理

1. redis 等、サーバーに新たなミドルウェアがインストールされる
1. isucontinuous.yaml に新たな import/deploy 対象を追記
1. 1台目の対象サーバ上で`import` : remote branchとの同期、対象ファイルの取得、commit、pushを一度に実行

`import`の取得失敗時は取得済みファイルが作業ツリーに、push失敗時はcommitがローカルに残ります。原因を解消した後は、必要に応じて単独の`sync`または`push`で復旧します。
