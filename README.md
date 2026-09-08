# CLI Quest

CLI Questは、実務のトラブルを安全な専用Workspaceへ再現し、自分でterminalを操作して解決するGo製CLI学習アプリです。

コマンド名を答えるクイズではありません。`ls`、`find`、`git`、`ps`、`curl`などを自由に組み合わせ、file permission、Git history、process、HTTP requestといった**最終状態**を作ることでSceneをClearします。入力したcommand stringそのものは採点しません。

## v0.2.0の主な機能

- Linux、Git、Process、HTTP、Shell、Network、Incidentの24 Scene
- 分野・難易度・Clear状態による絞り込みと対話式Scene選択
- `next`、`cancel`、`cleanup`、`open`、`solution`
- Clear後の原因、別解、実務上の注意点
- Easy/Normal/HardとHint数に応じたXP
- 挑戦回数、Clear時間、Hint使用数、履歴、連続学習日数
- 日本語・英語切り替え、JSON出力、shell completion
- 再帰的な`all`/`any`複合Validator
- Scene JSON Schemaと`scene validate`
- macOS/Linux CI、tagからの自動GitHub Release、checksum、Homebrew Formula
- file lock、stale resource検出、Workspace外symlink拒否を含む安全設計

## 必要な環境

- macOSまたはLinux
- Git（Git Scene）
- curl（HTTP Scene）
- jq（`http-json`で推奨）
- Go 1.23以上（sourceからbuildする場合）

Dockerは必須ではありません。環境は次で確認できます。

```bash
cliquest doctor
```

## インストール

### Goでインストール

```bash
go install github.com/tensho1026/CLI-Quest/cmd/cliquest@latest
export PATH="$(go env GOPATH)/bin:$PATH"
```

### GitHub Releaseからインストール

[Releases](https://github.com/tensho1026/CLI-Quest/releases)からOS/architectureに合うarchiveをdownloadし、`checksums.txt`でSHA-256を確認します。

```bash
sha256sum -c checksums.txt                 # Linux
shasum -a 256 -c checksums.txt             # macOS
tar -xzf cliquest_v0.2.0_darwin_arm64.tar.gz
install -m 0755 cliquest_v0.2.0_darwin_arm64/cliquest "$HOME/.local/bin/cliquest"
```

### Homebrew Formulaを使う

```bash
brew install --formula https://raw.githubusercontent.com/tensho1026/CLI-Quest/main/Formula/cliquest.rb
```

Formulaはrelease tagのsourceをGoでbuildします。

### Repositoryからbuild

```bash
git clone https://github.com/tensho1026/CLI-Quest.git
cd CLI-Quest
go build -o cliquest ./cmd/cliquest
./cliquest version
```

## はじめ方

### 対話形式で選ぶ

IDを指定せずに開始すると、分野、難易度、Sceneを順番に選択できます。

```bash
cliquest start

Choose a category
  0) All
  1) Git
  2) HTTP
  3) Incident
  4) Linux
  ...
```

### Filterして選ぶ

```bash
cliquest list --category git
cliquest list --difficulty hard
cliquest list --category http --difficulty normal --status uncleared

cliquest start --category git --difficulty hard
cliquest next --category linux --difficulty easy
```

指定できる値:

- `--category`: `linux`、`git`、`process`、`http`、`shell`、`network`、`incident`
- `--difficulty`: `easy`、`normal`、`hard`
- `--status`: `clear`、`active`、`uncleared`（`list`のみ）

### Sceneを解く

```bash
cliquest start linux-permission
cliquest open
cd ~/.cliquest/workspaces/linux-permission

ls -l deploy.sh
chmod u+x deploy.sh
cliquest check
```

`check`はWorkspace内でも別directoryからでも実行できます。Clearすると、原因、別解、注意点、獲得XPが表示されます。

```text
✓ CLEAR

What happened:
The file content was valid, but no executable permission bit was set.

Other solutions:
  - chmod +x deploy.sh
  - chmod u+x deploy.sh

Be careful:
chmod 777 grants unnecessary access and is rarely the right fix.

+100 XP
```

## コマンド

| Command | 説明 |
| --- | --- |
| `cliquest` | Home、全体進捗、XP、streakを表示 |
| `cliquest list [filters]` | Sceneを絞り込んで一覧表示 |
| `cliquest start [scene-id] [filters]` | Sceneを選択・開始 |
| `cliquest next [filters]` | 条件に合う次の未Clear Sceneを開始 |
| `cliquest check` | 現在の状態をValidatorで確認 |
| `cliquest hint` | 次の段階的Hintを表示 |
| `cliquest solution` | 解答例を明示表示。最大Hint penaltyを適用 |
| `cliquest reset` | Active Sceneを初期状態から再作成 |
| `cliquest cancel` | 所有resourceを停止しActive Workspaceを削除 |
| `cliquest cleanup` | orphan helperとstale resource recordを掃除 |
| `cliquest open` | Active Workspaceへ移動する`cd` commandを表示 |
| `cliquest status` | Active Scene、進捗、XPを表示 |
| `cliquest history [--limit N]` | Clear/cancel履歴、時間、Hint、XPを表示 |
| `cliquest stats` | 分野・難易度別の成績を表示 |
| `cliquest doctor` | Git、curl、jq、Dockerを確認 |
| `cliquest config language en\|ja` | 表示言語を保存 |
| `cliquest completion bash\|zsh\|fish` | shell completionを生成 |
| `cliquest scene validate FILE` | 外部Scene JSONを検証 |
| `cliquest version` | Version表示 |

### `reset`、`cancel`、`cleanup`の違い

- `reset`: 現在のWorkspaceを削除し、同じSceneを直ちに再作成します。attempt数も増えます。
- `cancel`: 現在の所有processを停止しWorkspaceを削除して、Active Sceneを解除します。
- `cleanup`: Active Sceneは維持し、過去の異常終了などで残ったorphan processやstale registryだけを掃除します。

Workspaceで作ったfileが必要なら、`reset`や`cancel`の前に退避してください。

## XP・履歴・統計

基本XPはEasy 100、Normal 150、Hard 250です。Hint 1回につき20 XP減り、最低値は基本XPの40%です。`solution`を表示すると最大Hint penaltyになります。再挑戦でもXPとClear履歴は記録されますが、全体のClear Scene数は重複しません。

```bash
cliquest history --limit 10
cliquest stats
```

記録内容:

- Sceneごとの開始・終了日時
- attempt数、Clear数
- Clear/cancelと所要時間
- Hint使用数
- Scene・category別XP
- 難易度別達成率
- consecutive completion days

## 日本語・英語

設定を保存する場合:

```bash
cliquest config language ja
cliquest config language en
```

一回だけ上書きする場合:

```bash
cliquest --lang ja start linux-permission
cliquest --lang en list
```

全24 Sceneに日本語textがあります。未翻訳fieldが将来追加された場合はEnglishへfallbackします。

## JSON出力

`--json`はcommandの前後どちらでも指定できます。

```bash
cliquest --json list --category git
cliquest stats --json
cliquest check --json
```

`list`、`start`、`check`、`hint`、`reset`、`cancel`、`cleanup`、`open`、`status`、`history`、`stats`、`doctor`、`config`、`scene validate`、`version`がmachine-readable outputに対応しています。

## Shell completion

### zsh

```bash
mkdir -p ~/.zfunc
cliquest completion zsh > ~/.zfunc/_cliquest
fpath=(~/.zfunc $fpath)
autoload -Uz compinit && compinit
```

### bash

```bash
cliquest completion bash > ~/.cliquest-completion.bash
source ~/.cliquest-completion.bash
```

### fish

```bash
cliquest completion fish > ~/.config/fish/completions/cliquest.fish
```

## 収録Scene

| ID | 分野 | 難易度 | 主な学習内容 |
| --- | --- | --- | --- |
| `linux-permission` | Linux | Easy | chmod、実行権限 |
| `linux-find-file` | Linux | Normal | find、探索 |
| `linux-disk-usage` | Linux | Normal | du、directory容量 |
| `linux-log-search` | Linux | Normal | grep、log調査 |
| `disk-large-files` | Linux | Normal | find、du、巨大file |
| `permissions-owner` | Linux | Normal | ownership、mode 0600 |
| `git-restore-file` | Git | Easy | 特定fileのrestore |
| `git-wrong-branch` | Git | Normal | branch、reset |
| `git-lost-commit` | Git | Hard | reflog、commit復旧 |
| `git-conflict` | Git | Normal | conflict解消、merge commit |
| `git-stash-recovery` | Git | Normal | stash復旧 |
| `git-bisect` | Git | Hard | regression commit特定 |
| `port-conflict` | Process | Easy | lsof、ps、kill |
| `runaway-process` | Process | Normal | CPU process調査 |
| `process-log` | Process | Hard | logとportの横断調査 |
| `http-health-check` | HTTP | Easy | curl、health endpoint |
| `http-auth` | HTTP | Normal | Authorization header |
| `http-json` | HTTP | Normal | curl、jq、JSON抽出 |
| `http-debug` | HTTP | Normal | status、header、redirect |
| `shell-pipeline` | Shell | Normal | grep、pipe、wc、awk |
| `env-missing` | Shell | Easy | environment file |
| `path-broken` | Shell | Normal | PATH、symbolic link |
| `network-dns` | Network | Normal | DNSとhosts override |
| `api-database-incident` | Incident | Hard | HTTP、log、configの切り分け |

## Process/HTTP Scene

使用portは8080、18080〜18085です。開始前から別applicationがportを使用している場合、CLI Questはそのprocessを終了せずScene開始を失敗させます。

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
ps -p <PID> -o pid,ppid,%cpu,command
```

`kill`前にcommandへ`cliquest`と現在のWorkspaceが含まれることを確認してください。CLI Quest自身のCleanupも、PID、random token、Workspaceがすべて一致するprocessだけを停止します。

## Data保存場所

```text
~/.cliquest/
├── progress.json
├── progress.lock
├── history.jsonl
└── workspaces/
    └── <scene-id>/
```

`progress.json`にはActive Scene、Clear一覧、attempt statistics、XP、languageを保存します。挑戦履歴は`history.jsonl`へ分離して1行1entryで追記するため、Hintや設定変更のたびに過去の履歴全体を書き直しません。既存の`progress.json`にinline保存されたhistoryも読み込み可能で、state更新時に自動移行します。`progress.lock`のUnix file lockとatomic renameにより、同時実行時のlost updateと途中書込みを防ぎます。

保存先を分離する場合:

```bash
CLIQUEST_HOME=/tmp/my-cliquest cliquest list
```

以降のcommandでも同じ`CLIQUEST_HOME`を指定してください。

## 安全性

- Setup/Reset対象をCLI Quest専用Workspaceへ限定
- ユーザーの既存file、Git repository、system設定を変更しない
- `sudo`を要求しない
- PID、token、Workspaceが一致する所有processだけを自動停止
- 既に使用中のportを奪わない
- ValidatorがWorkspace外を指すsymlinkを拒否
- Scene ID、relative path、Setup、Validator typeを起動前に検証
- 入力commandを実行・採点せず最終状態を確認
- `cleanup`で異常終了後のstale registryを検出

ユーザーがWorkspaceで入力するcommand自体をsandbox化するものではありません。削除やprocess終了前には`pwd`と対象を確認してください。

## Scene開発

定義は`scenes/<category>/<scene-id>.json`へ置き、`go:embed`でbinaryに同梱します。Schemaは[schemas/scene.schema.json](schemas/scene.schema.json)です。

```bash
cliquest scene validate scenes/linux/linux-permission.json
```

複合Validator例:

```json
{
  "type": "all",
  "validators": [
    {"type": "git_no_conflicts"},
    {"type": "git_clean"},
    {"type": "git_file_contains", "path": "app.conf", "value": "feature=true"}
  ]
}
```

`all`と`any`は再帰的にnestできます。汎用type:

- File: `file_exists`、`file_contains`、`file_not_contains`、`file_mode`、`executable`
- Git: `git_branch`、`git_clean`、`git_no_conflicts`、`git_file_contains`、`git_commit_contains`
- Resource: `process_stopped`、`port_available`、`marker_exists`

Sceneには`solution`、`lesson.what_happened`、`lesson.other_solutions`、`lesson.be_careful`、`translations.ja`を記述します。新しい汎用typeで複数Sceneを表現できる場合にだけGo側を拡張してください。

## 開発・Test

```bash
gofmt -w cmd/cliquest/main.go internal/*/*.go scenes/embed.go
go test -race ./...
go vet ./...
go build ./cmd/cliquest
GOOS=linux GOARCH=amd64 go build -o /tmp/cliquest-linux-amd64 ./cmd/cliquest
```

GitHub ActionsはUbuntuとmacOSの両方でformat、race test、vet、buildを実行します。`v*` tagをpushするとdarwin/linuxのamd64/arm64 archive、`checksums.txt`、release notesをGitHub Releaseへ自動公開します。

詳細設計は[docs/architecture.md](docs/architecture.md)を参照してください。

## License

MIT Licenseです。詳細は[LICENSE](LICENSE)を参照してください。
