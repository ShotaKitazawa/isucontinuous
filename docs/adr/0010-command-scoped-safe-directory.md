---
status: accepted
pr: nil
date: 2026-10-18
related: [ADR-0008]
---

# ADR-0010: `safe.directory` を Git コマンド単位で指定する

## 背景と課題

`import` は対象ディレクトリの未追跡ファイルを列挙するため、対象上で Git を実行する。従来は dubious ownership を回避するため、対象pathをglobal `safe.directory`へ永続的に追記していた。この変更はisucontinuousの実行後も残り、利用者のGit設定を増やし続ける。

## 判断の基準

- 利用者のglobal Git configを変更しない。
- 対象が既存repository、そのsubdirectory、repository外のいずれでも現在の列挙結果を維持する。
- 対象directoryに `.git` を作成または削除しない。

## 検討した選択肢

| 案 | 概要 | 採らなかった理由 |
|---|---|---|
| global configへ追記 | `git config --global --add safe.directory` を実行する | 永続的な副作用があり、設定が蓄積する |
| `safe.directory`を使わない | dubious ownershipを無視して通常のGit commandを実行する | ownershipが異なる既存repositoryを扱えない |
| command scopeで指定 | `git -c safe.directory=...` を対象commandだけに付ける | 採用 |

## 決定

採用: **command scopeで指定する**。Gitの `-c` によるcommand scopeはprotected configurationとして扱われ、`safe.directory`に有効である。dubious ownership保護がない古いGitでは、そもそもこの設定を必要としない。

### 詳細

1. repository rootの検出だけは `git -c safe.directory='*' rev-parse --show-toplevel` を使う。
2. 既存repository内では、検出したrootを `git -c safe.directory='<root>' ...` と後続commandに限定して指定する。
3. repository外では従来どおり一時 `--git-dir` と対象 `--work-tree` を使う。この一時repositoryには `safe.directory` を指定しない。
4. pathはshell引数としてquoteし、global configの読み書きは行わない。

## 影響

- 良い影響: `import`を繰り返してもglobal Git configが変化しない。
- 悪い影響・トレードオフ: root検出commandに限って `safe.directory='*'` を許可する。ただし実行するGit操作はroot pathの取得のみである。
- 他のADR・コードへの影響: ADR-0008 C4のglobal追記方針を置き換える。

## 未決事項

なし。

## 実装時に決めたこと

- 一時 `--git-dir` とcleanup pathも同じshell quoting関数を使い、spaceやsingle quoteを含むpathを扱えるようにした。
