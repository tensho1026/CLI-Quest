# CLI Quest

CLI Questは、実務で遭遇するトラブルを安全な専用Workspace内に再現し、自分でターミナルを操作して解決するGo製CLI学習アプリです。

コマンド名を答えるクイズではありません。`ls`、`find`、`git`、`ps`、`curl`などを自由に組み合わせ、ファイル権限、Git履歴、プロセス、HTTPリクエストといった**最終状態**を作ることでSceneをクリアします。入力したコマンド文字列そのものを正解判定には使いません。

## 主な特徴

- Linux、Git、Process、HTTPの実務的な11 Scene
- Sceneごとに独立した `~/.cliquest/workspaces/<scene-id>` を生成
- 何度でも初期状態へ戻せる `reset`
- 段階的に3つまで表示するHint
- ローカルJSONによるクリア状況の保存
- macOSとLinuxに対応するシンプルな単一バイナリ
- CLI Questが作成したWorkspaceとプロセスだけを後始末する安全設計
- JSON定義を中心にSceneを追加できる構成

## 必要な環境

- Go 1.23以上（ソースからインストールする場合）
- macOSまたはLinux
- Git（Git Sceneで使用）
- curl（HTTP Sceneで推奨）

`jq`とDockerはMVPの必須依存ではありません。利用できるコマンドは次で確認できます。

```bash
cliquest doctor
```

## インストール

### `go install`を使う

Goがインストール済みなら、次のコマンドでインストールできます。

```bash
go install github.com/tensho1026/CLI-Quest/cmd/cliquest@latest
```

通常、バイナリは `$(go env GOPATH)/bin/cliquest` に作成されます。コマンドが見つからない場合は、そのディレクトリを `PATH`へ追加してください。

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### リポジトリからビルドする

```bash
git clone https://github.com/tensho1026/CLI-Quest.git
cd CLI-Quest
go build -o cliquest ./cmd/cliquest
./cliquest help
```

システムの任意の場所から実行したい場合は、ビルドした `cliquest` を `PATH`の通ったディレクトリへ配置します。

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 cliquest "$HOME/.local/bin/cliquest"
```

## まず1 Scene遊ぶ

Scene一覧を表示します。

```bash
cliquest list
```

最初は `linux-permission` がおすすめです。

```bash
cliquest start linux-permission
```

表示されたWorkspaceへ移動します。

```bash
cd ~/.cliquest/workspaces/linux-permission
```

状況を調べて解決します。たとえば、ファイルの現在の状態は次のように確認できます。

```bash
ls -l deploy.sh
./deploy.sh
```

必要な操作を行ったら、Workspace内からでも別のディレクトリからでも判定できます。

```bash
cliquest check
```

まだクリアできていない場合もWorkspaceはそのまま残ります。行き詰まったときは段階的なHintを表示できます。

```bash
cliquest hint
```

完全に最初からやり直す場合は次を実行します。

```bash
cliquest reset
```

`reset`は現在のSceneのWorkspaceを削除して作り直します。Workspaceに自分で追加したファイルも消えるため、必要な内容があれば先に別の場所へ保存してください。

## コマンド一覧

| コマンド | 説明 |
| --- | --- |
| `cliquest` | ホーム画面とカテゴリ別進捗を表示 |
| `cliquest list` | Scene ID、カテゴリ、難易度、状態を一覧表示 |
| `cliquest start <scene-id>` | Sceneの初期状態を作成して開始 |
| `cliquest check` | 現在のWorkspaceや関連リソースの状態を検証 |
| `cliquest hint` | 次のHintを表示し、使用回数を保存 |
| `cliquest reset` | Active Sceneを初期状態から再作成 |
| `cliquest status` | 全体進捗、Active Scene、Workspaceを表示 |
| `cliquest doctor` | Git、curl、jq、Dockerの有無を確認 |
| `cliquest help` | ヘルプを表示 |

新しいSceneを `start`すると、それまでActiveだったSceneの管理対象リソースを停止し、そのWorkspaceを削除してから切り替えます。同時に進められるActive Sceneは1つです。クリア済みの記録は消えません。

## 収録Scene

| ID | カテゴリ | 難易度 | 学べる内容 |
| --- | --- | --- | --- |
| `linux-permission` | Linux | Easy | `ls -l`、`chmod`、実行権限 |
| `linux-find-file` | Linux | Normal | `find`、ディレクトリ探索 |
| `linux-disk-usage` | Linux | Normal | `du`、容量比較 |
| `linux-log-search` | Linux | Normal | `grep`、ログ調査 |
| `git-restore-file` | Git | Easy | `git status`、`git diff`、`git restore` |
| `git-wrong-branch` | Git | Normal | branch、commit、reset、履歴の保全 |
| `git-lost-commit` | Git | Hard | `git reflog`、失われたcommitの復旧 |
| `port-conflict` | Process | Easy | `lsof`、`ps`、`kill`、port調査 |
| `runaway-process` | Process | Normal | CPU使用プロセスの特定と終了 |
| `http-health-check` | HTTP | Easy | `curl`、GET、health endpoint |
| `http-auth` | HTTP | Normal | HTTP Header、Bearer認証 |

### Process Sceneについて

`port-conflict`は `127.0.0.1:8080`、HTTP Sceneは `127.0.0.1:18080`または`127.0.0.1:18081`だけを使用します。Scene開始前から対象portが他のアプリに使われている場合、CLI Questはそのプロセスを終了せず、安全のためScene開始を失敗させます。既存アプリを自分で停止するか、利用終了後にもう一度 `start`してください。

Process SceneでPIDを調べる例は次のとおりです。

```bash
# macOS / lsofがあるLinux
lsof -nP -iTCP:8080 -sTCP:LISTEN

# コマンド内容の確認
ps -p <PID> -o pid,ppid,%cpu,command
```

`kill`する前に、コマンドに `cliquest`と現在のWorkspaceが含まれることを必ず確認してください。CLI Questの `reset`やScene切り替え処理も、記録したPIDとランダムな起動トークンの両方が一致したプロセスだけを停止します。

## データの保存場所

デフォルトでは次の場所を使います。

```text
~/.cliquest/
├── progress.json
└── workspaces/
    └── <scene-id>/
```

- `progress.json`: クリア済みScene、Active Scene、Hint使用数、管理対象リソース
- `workspaces/<scene-id>`: 自由に調査・編集する演習環境

テスト用などで保存場所を分離したい場合は `CLIQUEST_HOME`を指定できます。

```bash
CLIQUEST_HOME=/tmp/my-cliquest cliquest list
```

この環境変数を使った場合、その後の `start`、`check`、`hint`などでも同じ値を指定してください。値が異なると別の進捗として扱われます。

## 安全性

CLI Questは次の方針でユーザー環境を保護します。

- 演習ファイルはCLI Quest専用Workspace内だけに作成
- ユーザーの既存GitリポジトリをSetupやResetの対象にしない
- `sudo`やシステム設定の変更を要求しない
- Resetで削除するパスを `workspaces/<scene-id>`に限定
- CLI Quest自身が起動し、PIDとトークンを記録したプロセスだけを自動停止
- 既に利用中のportを奪わず、Scene開始を安全に失敗させる
- 入力コマンドを実行・採点せず、Workspaceなどの最終状態だけを検証

CLI操作を学ぶアプリなので、ユーザーがWorkspace内で入力するコマンド自体はCLI Questがsandbox化しません。`pwd`で現在地を確認し、特に削除やプロセス終了を行う前には対象を確認してください。

## よくある問題

### `cliquest: command not found`

`go install`先が `PATH`に含まれているか確認します。

```bash
go env GOPATH
echo "$PATH"
```

### Git Sceneを開始できない

`cliquest doctor`でGitが `✓`か確認してください。Git SceneはWorkspace内に完全に独立した練習用リポジトリを作ります。グローバルのuser.nameやuser.emailは使わず、練習用リポジトリ内だけに専用値を設定します。

### Process/HTTP Sceneを開始できない

エラーに `address already in use`と表示された場合は、対象portを別のアプリが使用しています。CLI Questは既存プロセスを自動終了しません。`lsof`や`ss`で所有者を確認してください。

### 進捗を完全に初期化したい

CLI Questに「全進捗削除」コマンドはありません。誤操作を避けるためです。Process/HTTP SceneがActiveなら、まず `cliquest start linux-permission`のようにhelperを使わないSceneへ切り替えて、管理対象processを終了させてください。その後 `~/.cliquest`の中身を自分で確認して削除します。Workspaceの必要なファイルは先に退避してください。

## 開発

```bash
git clone https://github.com/tensho1026/CLI-Quest.git
cd CLI-Quest

go test ./...
go vet ./...
go build ./cmd/cliquest
```

Linux向けにcompileできることだけを確認する場合は次を使えます。

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/cliquest-linux-amd64 ./cmd/cliquest
```

設計の詳細は [docs/architecture.md](docs/architecture.md) を参照してください。

## Sceneを追加する

Scene定義は `scenes/<category>/<scene-id>.json`に置きます。ファイルを追加すると `go:embed`によってバイナリへ同梱されるため、実行時にリポジトリやカレントディレクトリは不要です。

汎用ファイルSceneの最小例です。

```json
{
  "id": "linux-example",
  "title": "Example",
  "category": "linux",
  "difficulty": "easy",
  "description": "調査する状況の説明",
  "mission": "ユーザーが作るべき最終状態",
  "success": "クリア時の説明",
  "setup": {
    "type": "filesystem",
    "files": [
      {"path": "result.txt", "content": ""}
    ]
  },
  "validation": {
    "type": "file_contains",
    "path": "result.txt",
    "value": "expected state"
  },
  "hints": ["Hint 1", "Hint 2", "Hint 3"]
}
```

現在利用できる汎用Validatorは `executable`と`file_contains`です。Git、Process、HTTPの特殊Setup/ValidatorはGo側に実装されています。新しい汎用型を増やす場合は、定義だけで複数Sceneを作れる粒度を優先してください。

追加後は必ず次を実行し、MVPカテゴリ数、ID重複、主要CLIフローのテストも通ることを確認します。

```bash
gofmt -w cmd/cliquest/main.go internal/*/*.go scenes/embed.go
go test ./...
go vet ./...
```

## ライセンス

MIT Licenseです。詳細は [LICENSE](LICENSE) を参照してください。
