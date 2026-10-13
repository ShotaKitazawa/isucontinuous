# AGENTS.md

## ADR (`docs/adr/`)

設計上の決定は ADR として `docs/adr/` に日本語で書く。

- 書式は [`docs/adr/template.md`](docs/adr/template.md) (MADR をもとにした形) に従う。節の順序と名前を変えない。
- ファイル名は `NNNN-<kebab-case>.md`。番号は欠番が出ても詰めない。
- front matter の `status` は `proposed | accepted | deprecated | superseded by ADR-NNNN`。
- front matter の `pr` には、実装に着手した PR の番号 (`"#NN"`) を書く。未着手は `nil`。
- 採用済み (`accepted`) の「決定」は書き換えない。変えるときは新しい ADR を作り、旧 ADR を `superseded by` にする。
- 実装する PR が編集してよいのは、`pr`、「未決事項」のチェック、「実装時に決めたこと」だけ。
- ADR を追加したら `docs/adr/README.md` の一覧に行を足す。状態と PR は各 ADR の front matter が正で、README には書かない。
