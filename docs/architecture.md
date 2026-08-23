# CLI Quest Architecture

## 設計目標

CLI QuestのMVPは、安全性、Scene追加の容易さ、単純さ、テスト可能性、CLI UXの順で設計しています。高度なTUI、database、plugin systemは持ちません。

## コンポーネント

```text
cmd/cliquest/main.go
        │
        ▼
internal/cli          command dispatch / output
   ├── internal/scene       embedded JSON catalog
   ├── internal/progress    atomic JSON persistence
   ├── internal/workspace   setup / reset / owned helpers
   └── internal/validator   state-based clear checks
        ▲
        │
scenes/<category>/*.json
```

### `internal/scene`

埋め込まれたJSONを起動時に読み込み、必須field、IDの安全性、重複を確認します。Sceneは説明、Mission、Setup、Validation、Hintを1つの定義にまとめます。

### `internal/workspace`

`<CLIQUEST_HOME>/workspaces/<scene-id>`の作成と初期状態生成を担当します。filesystem型はJSONに記述したファイル、内容、mode、sizeだけでSetupできます。Git型は練習用リポジトリを初期化します。

Process/HTTP型は現在実行中の `cliquest`バイナリを内部helper commandで再起動します。起動時に暗号学的乱数からtokenを作り、PID、token、helper種別をprogressへ保存します。Cleanupは`ps`でPIDのcommand lineにtokenが残っていることを確認してからsignalを送るため、PIDが再利用された場合に無関係なprocessを終了しません。

### `internal/validator`

Validatorはユーザーが入力したcommand stringを受け取りません。次の状態だけを読み取ります。

- Unix file mode
- answer fileの内容
- Git branch/ref/tree/diff
- 管理対象PIDの生存状態
- localhost portをbindできるか
- HTTP helperが正しいrequestを受信して作成したmarker

これにより、特定の解法を強制せず、同じGoalへ到達する複数の方法を許容します。

### `internal/progress`

進捗は単一の`progress.json`です。一時ファイルへ書いてcloseした後にrenameすることで、途中終了時に壊れたJSONが残りにくいatomic saveにしています。file modeは0600、状態directoryは0700です。

## Lifecycle

### Start

1. 既存Active Sceneがあれば所有resourceをCleanup
2. 既存Active Workspaceだけを削除
3. 新しいWorkspaceと初期状態をSetup
4. Active Scene、Hint数、resource情報をprogressへ保存

Setup途中で失敗した場合は、そのSetupが作成したresourceとWorkspaceをCleanupして終了します。

### Check

1. progressからActive Sceneを取得
2. 対応するValidatorで現在状態を検査
3. 未達ならWorkspaceを維持して結果を表示
4. Clearなら所有helperをCleanupし、completedへScene IDを重複なく追加
5. Active Sceneを解除してprogressを保存

### Reset

1. Active Sceneの所有helperをCleanup
2. Active Workspaceを限定的に削除
3. 同じSceneをSetupし直す
4. Hint数を0にしてActive情報を保存

## Portability

filesystem、Git、HTTPはGo標準libraryとGit CLIでmacOS/Linux共通です。Processの生存確認と終了にはUnix signalを使用し、command確認には両OSにある`ps -p <pid> -o command=`を使います。WindowsはMVP対象外です。

Sceneで学ぶ調査commandはOSによって異なる場合があります。HintではmacOSと多くのLinuxで使える`lsof`を案内し、READMEでは環境に応じて`ss`などを使えることも説明しています。

## Trust boundary

Scene定義はapplicationと一緒にcompileされるtrusted inputです。それでもIDとrelative pathは検査し、Workspace外を指すabsolute pathや`..`を拒否します。ユーザー操作そのものはsandboxではないため、CLI表示と文書でWorkspace確認を促します。
