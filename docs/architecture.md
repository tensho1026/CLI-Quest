# CLI Quest Architecture

## 設計目標

CLI Quest v0.2は、安全性、Scene追加の容易さ、学習feedback、テスト可能性、CLI UXの順で設計しています。高度なTUIやdatabaseは持たず、単一binaryとlocal JSONで完結します。

## コンポーネント

```text
cmd/cliquest/main.go
        │
        ▼
internal/cli          command dispatch / selection / output / i18n
   ├── internal/scene       embedded JSON catalog
   ├── internal/progress    atomic JSON persistence
   ├── internal/workspace   setup / reset / owned helpers
   └── internal/validator   state-based clear checks
        ▲
        │
scenes/<category>/*.json
```

### `internal/scene`

埋め込まれたJSONを起動時にstrict decodeし、unknown field、必須field、ID、relative path、Setup/Validator type、重複を確認します。Sceneは説明、Mission、Setup、再帰的Validation、Hint、Solution、Lesson、translationを1つの定義にまとめます。外部定義にも同じ`scene.Decode`を使うため、`scene validate`と実行時で判定がずれません。

### `internal/workspace`

`<CLIQUEST_HOME>/workspaces/<scene-id>`の作成と初期状態生成を担当します。filesystem型はJSONに記述したファイル、内容、mode、sizeだけでSetupできます。Git型は練習用リポジトリを初期化します。

Process/HTTP型は現在実行中の`cliquest` binaryを内部helper commandで再起動します。起動時に暗号学的乱数からtokenを作り、PID、token、helper種別、WorkspaceをprogressとWorkspace内registryへ保存します。Cleanupは`ps`でPIDのcommand lineにtokenとWorkspaceが残っていることを確認してからsignalを送るため、PID再利用時に無関係なprocessを終了しません。`cleanup`はregistryをscanし、Active Sceneを維持したままorphan helperとdead PID recordだけを処理します。

### `internal/validator`

Validatorはユーザーが入力したcommand stringを受け取りません。次の状態だけを読み取ります。

- Unix file mode
- answer fileの内容
- Git branch/ref/tree/diff
- 管理対象PIDの生存状態
- localhost portをbindできるか
- HTTP helperが正しいrequestを受信して作成したmarker
- 複数条件の`all` / `any`

これにより、特定の解法を強制せず、同じGoalへ到達する複数の方法を許容します。

File ValidatorはWorkspace rootとtargetのsymlinkを解決し、resolved pathがWorkspace外なら拒否します。Git commandもCLI QuestがSetupしたActive Workspaceだけをworking directoryにします。

### `internal/progress`

進捗metadataは`progress.json`、挑戦履歴はappend-onlyの`history.jsonl`へ分離します。Hintや設定変更のように履歴を変更しない操作では、履歴journalをdecodeせず、進捗metadataだけをatomic saveします。Clear/cancel時は新しい履歴entryだけをjournalへappendするため、過去の履歴全体を再encodeしません。

`progress.lock`へUnix advisory lockを取り、metadataとhistory journalのread-modify-write transaction中のlost updateを防ぎます。一時fileへ書いてcloseした後にrenameするatomic saveも併用します。modeは0600、状態directoryは0700です。Version 0.1/0.2のように`progress.json`へhistoryがinline保存されたデータは読み込み可能で、次回のstate update時にjournalへ移行します。

Version 0.1の`completed`と`active`だけのJSONもそのままdecodeし、missing mapやlanguageへdefaultを設定します。v0.2ではattempt、Clear/cancel history、duration、Hint、XP、best time、settingsを追加しています。

## Lifecycle

### Start

1. 既存Active Sceneがあれば所有resourceをCleanup
2. 既存Active Workspaceだけを削除
3. 新しいWorkspaceと初期状態をSetup
4. attempt数を増やし、Active Scene、Hint数、resource情報をprogressへ保存

Setup途中で失敗した場合は、そのSetupが作成したresourceとWorkspaceをCleanupして終了します。

### Check

1. progressからActive Sceneを取得
2. 対応するValidatorで現在状態を検査
3. 未達ならWorkspaceを維持して結果を表示
4. Clearなら所有helperをCleanupし、completedへScene IDを重複なく追加
5. 難易度とHint数からXPを計算し、historyとScene statsへ記録
6. Active Sceneを解除してprogressを保存

### Reset

1. Active Sceneの所有helperをCleanup
2. Active Workspaceを限定的に削除
3. 同じSceneをSetupし直す
4. Hint数を0にしてActive情報を保存

Reset前のattemptはcancelled historyになり、新しいattemptとしてcountします。`cancel`は再SetupせずActive Workspaceを削除します。

## Selection and presentation

Catalog filterはcategory、difficulty、completion stateを組み合わせます。`start`にIDがなければinput readerを使う段階的selectorへ進み、`next`は同じfilterから最初の未Clear Sceneを選びます。Input readerはinject可能なためinteractive flowもunit testできます。

Scene textはEnglish definitionをbaseとし、`translations.<language>`をfield単位でoverlayします。missing translationはEnglishへfallbackします。JSON modeはANSI colorを無効にし、structまたはmapをencoderへ渡します。通常表示のcolorもTTYかつ`NO_COLOR`未設定の場合だけ有効です。

## Release pipeline

CIはGitHub-hosted Ubuntu/macOSでrace test、vet、format、buildを実行します。`v*` tag workflowはCGOを無効にしてdarwin/linux × amd64/arm64をcross compileし、archiveとSHA-256 checksumをGitHub CLIでReleaseへ公開します。Homebrew Formulaは同じtagのsourceをGoでbuildします。

## Portability

filesystem、Git、HTTPはGo標準libraryとGit CLIでmacOS/Linux共通です。Processの生存確認と終了にはUnix signalを使用し、command確認には両OSにある`ps -p <pid> -o command=`を使います。WindowsはMVP対象外です。

Sceneで学ぶ調査commandはOSによって異なる場合があります。HintではmacOSと多くのLinuxで使える`lsof`を案内し、READMEでは環境に応じて`ss`などを使えることも説明しています。

## Trust boundary

Scene定義はapplicationと一緒にcompileされるtrusted inputです。それでもIDとrelative pathは検査し、Workspace外を指すabsolute pathや`..`を拒否します。ユーザー操作そのものはsandboxではないため、CLI表示と文書でWorkspace確認を促します。
