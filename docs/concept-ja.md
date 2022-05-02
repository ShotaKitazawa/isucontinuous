# Concept

isucontinuous は複数台のサーバを用いた複数人での開発を支援するツールです。
isucontinuous は複数台のサーバ上にあるソースコードを取り込み、継続的なデプロイメント及びプロファイリングを実現します。

isucontinuous には以下のそれぞれのフェーズを意識したコマンド体系が用意されています。

* `init`: Git ローカルリポジトリを初期化する

![init](./images/init.jpg?raw=true)

* `setup`: 開発に必要なソフトウェアをインストールする

![setup](./images/setup.jpg?raw=true)

* `import`: サーバ上の任意のファイルを取得し Git ローカルリポジトリの管理下にコピーする

![import](./images/import.jpg?raw=true)

* `push`: Git ローカルリポジトリの更新を Git リモートリポジトリにプッシュする

![push](./images/push.jpg?raw=true)

* `sync`: Git リモートリポジトリの内容を Git ローカルリポジトリに反映する

![sync](./images/sync.jpg?raw=true)

* `deploy`: 任意のリビジョンにおける Git リモートリポジトリ管理下のファイルをサーバ上にデプロイする

![deploy](./images/deploy.jpg?raw=true)

* `profile`: 各サーバにて任意のコマンドを実行する

![profiling](./images/profiling.jpg?raw=true)

* `afterbench`: 各サーバ上にて任意のコマンドを実行しプロファイルデータを生成後、指定したディレクトリ以下のファイルを Slack に POST する

![afterbench](./images/afterbench.jpg?raw=true)

## Usecase

### 1. 初期セットアップ

1. `init` : ローカルリポジトリの新規作成
1. isucontinuous.yaml の編集
1. `setup` : 各種開発用ソフトウェアのインストール
1. `import` : 各種設定ファイルやソースコードを Git で管理
1. `push` : GitHub に push

### 2. 開発・デプロイ

1. GitHub に push したリポジトリを元に各メンバーが各ブランチで作業
1. `deploy` : 特定リビジョンの各ファイルを各サーバにデプロイ
1. `profiling` : 各サーバにてプロファイリング用コマンドを実行
1. `afterbench` : プロファイルデータを収集し Slack に送信

### 3. 開発中サーバ上の新たなファイルを Git で管理

1. redis 等、サーバーに新たなミドルウェアがインストールされる
1. isucontinuous.yaml に新たな import/deploy 対象を追記
1. `sync` : ローカルリポジトリを remotes/origin/master と同期
1. `import` : 各種設定ファイルやソースコードを Git で管理
1. `push` : GitHub に push

