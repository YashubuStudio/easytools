# EasyTools（Legacy Exec GUI）仕様書 / 企画書

## 1. 概要（Purpose）

レガシーなCLI・バッチ・スクリプトを安全にラップし、**HTTP API化**して社内・ローカル利用を容易にするデスクトップ単体アプリ。
GUIから**ツール登録／編集／実行テスト**、および**内蔵HTTPサーバの起動・停止**を行えます。設定はYAMLにインポート／エクスポート可能。

* **モジュール**: `github.com/yashubustudio/easytools`
* **Go**: `go 1.24.5`
* **主要依存**:

  * `fyne.io/fyne/v2`（v2.6系想定）… クロスプラットフォームGUI
  * `gopkg.in/yaml.v3` … 設定YAML I/O

---

## 2. 目標（Goals）

* レガシーツールを**ノーコード**でHTTPエンドポイント化
* **安全な実行**（タイムアウト・環境変数ホワイトリスト・入出力制限）
* **UI上での迅速なテスト**と**ログ可視化**
* 設定の**再利用性**（YAML）

---

## 3. 非目標（Non-Goals）

* 複数ユーザ同時アクセス前提の大規模常駐サーバ
* 認証基盤（OAuth等）の実装
* 分散実行・ジョブキュー等のオーケストレーション

---

## 4. 画面設計（UI / Layout）

### 4.1 ナビゲーション

* **左サイドバー（Collapse 可）**

  * 「Server / API（Home）」と「Tools（Registry）」のタブ切替
  * 開状態：テキスト＋アイコン、閉状態：アイコンのみ

### 4.2 Home（トップ）

**左右 2:1 の水平分割**。右側は**縦に Test Run → Server Logs**。

* **左（2/3）: Server / API**

  * 2カラムフォーム

    * 左フォーム: `Addr`, `BasePath`, `Run`, `Tools`
    * 右フォーム: `Reload`, `Health`, `API Key`, `CORS`
  * ボタン行: `Start Server` / `Stop Server` / `Open Health` / ステータス（`Server: running ...` など）
* **右（1/3）**

  * 上：**Test Run**（縦一列。右端“吹き抜け”イメージで上段）

    * `Tool` セレクト
    * `Params(JSON)` / `Env(JSON)` / `Stdin`
    * `POST /run` ボタン
    * `Result`（HTTPコード＋JSON表示）
  * 下：**Server Logs**（自動更新・改行折返し）

**初期ウィンドウサイズ**: 960×640（狭め）。リサイズ可。

### 4.3 Registry（登録・編集）

**上下で 2/3 : 1/3 の垂直分割**。

* **上（2/3）**：左右で 2/3 : 1/3

  * **左（2/3）: ツール入力フォーム（スクロール、Advancedの折りたたみ無し）**

    * `Name`, `Group`, `Cmd`, `Args(comma)`, `Workdir`,
      `Env(KEY=VAL per line)`, `AllowEnv(comma)`,
      `Timeout`, `MaxStdout`, `MaxStderr`, `Stdin(Allow)`
    * ヘッダボタン：`Add` / `Save` / `Delete` / `Import YAML` / `Export YAML`
  * **右（1/3）: Tools（by Group）**

    * **アコーディオン**：`Group` 単位で展開・格納可能
    * リストクリックでフォームへロード
* **下（1/3）: Quick CMD**

  * `Tool` セレクト（未選択時はフォームの `Name` を推測）
  * `Params(JSON)` / `Env(JSON)` / `Stdin`
  * `Quick Run(POST /run)` ＋ `Result`

---

## 5. ユースケース / フロー

### 5.1 サーバ起動

1. Home で `Addr`, `BasePath`, パス類を設定
2. `Start Server` クリック
3. ステータス更新（`Server: running on ...`）
4. `Open Health` で `/healthz`（例）をブラウザで開く

### 5.2 ツール登録

1. Registry → `Add` で雛形投入
2. `Name`, `Cmd`, `Args` 等を編集
3. `Save` で反映（**rename** の場合は旧キー削除→新キー追加）
4. 右のアコーディオンに反映。`Home/Test Run` セレクトにも反映

### 5.3 テスト実行

1. Home → `Test Run` で `Tool` 選択
2. 必要に応じて `Params(JSON)` / `Env(JSON)` / `Stdin` 入力
3. `POST /run`
4. `Result` に `HTTP 200/400` と実行結果（標準出力・標準エラー・ExitCode 等）

### 5.4 YAML I/O

* **Export**: `tools.yaml`（`api_key` と `tools` を含む）
* **Import**: 同形式を読み込み、GUIへ反映

---

## 6. データモデル

### 6.1 Tool

```yaml
Tool:
  group: string           # 論理グループ（アコーディオン表示用）
  cmd: string             # 実行ファイル
  args: []string          # 引数（"{{key}}" テンプレ展開対応）
  workdir: string         # 作業ディレクトリ
  env: {string: string}   # 固定付与する環境変数
  allow_env: []string     # リクエストから受け取って転送を許可するキー
  timeout: string         # "30s", "5m" 等
  max_stdout: int         # 標準出力バイト上限（既定=10MiB）
  max_stderr: int         # 標準エラー上限（既定=2MiB）
  allow_stdin: bool       # Stdinの送信可否
```

### 6.2 ServerConfig

```yaml
ServerConfig:
  addr: ":8080"
  base_path: "/v1"
  api_key: "devkey"
  cors: true
  tools: {string: Tool}
  paths:
    run: "/run"
    tools: "/tools"
    reload: "/reload"
    health: "/healthz"
```

### 6.3 RunRequest / Response（HTTP）

```json
// POST {base}{/run}
{
  "tool": "echo",
  "params": {"msg": "hello"},   // 引数テンプレの {{msg}} を置換
  "env": {"API_TOKEN": "xxx"},  // allow_env に含まれるキーのみ転送
  "stdin": "optional text"
}
```

```json
// 200 / 400
{
  "tool": "echo",
  "command": ["/usr/bin/echo", "hello"],
  "exit_code": 0,
  "stdout": "hello\n",
  "stderr": "",
  "duration_ms": 12,
  "timed_out": false,
  "started_at": "2025-09-10T07:42:22Z",
  "ended_at":   "2025-09-10T07:42:22Z"
}
```

---

## 7. API 仕様

| メソッド | パス                 | 概要              | 認証                 |
| ---- | ------------------ | --------------- | ------------------ |
| GET  | `{base}{/healthz}` | 健康チェック          | 不要                 |
| GET  | `{base}{/tools}`   | ツール名一覧（文字列配列）   | 要（X-API-Key）※CORS可 |
| POST | `{base}{/reload}`  | 将来拡張用（OKを返すダミー） | 要                  |
| POST | `{base}{/run}`     | ツール実行           | 要                  |

* **認証**: `X-API-Key: <value>` ヘッダ。空なら無効（オープン）。
* **CORS**: `Access-Control-Allow-Origin: *` 等を返却（GUI設定に依存）。
* **エラー**: 400（不正リクエスト／コマンド未検出／テンプレ未解決／実行失敗）、404（tool未登録）、401（API Key不一致）

---

## 8. 実行モデル / セキュリティ

* **テンプレ引数**: `args` 内の `{{key}}` を `params[key]` で置換。未解決が残る場合は400。
* **環境変数**: `env`（固定）＋ `allow_env` に合致する `req.env` のみ転送。`PATH`等の既定安全変数は常に付与。
* **タイムアウト**: `timeout`（既定60s）。超過は`context.DeadlineExceeded`として終了（`exit_code=124`）。
* **出力制限**: 標準出力/標準エラーの上限バイトを**クリップ**（DoS抑止）。
* **作業ディレクトリ**: 必要時のみ設定。
* **API Key**: 簡易な共有鍵。LAN内利用を想定。

---

## 9. ロギング

* **GUI**にリングバッファ（最大 2,000行想定）で流し込み、300ms間隔で反映
* HTTPアクセログ（メソッド/パス/処理時間）とサーバ状態を出力

---

## 10. 永続化

* **アプリ設定**（`Addr`, `BasePath`, パス群, `api_key`, `cors`）は Fyne `Preferences` に保存
* **ツール群**は **YAML** を入出力（`tools.yaml`）。起動時、同名ファイルがあれば自動ロード

---

## 11. スレッド/UI更新規約（Fyne）

* **ネットワーク/外部実行は goroutine** で実施
* **UI更新は必ず `fyne.Do(func(){ ... })`**
  例: HTTPレスポンス受信後に `testOut.SetText(...)` する箇所など

---

## 12. ビルド/配布

* クロスプラットフォーム: Windows / macOS / Linux
* 例:

  ```bash
  go mod tidy
  go build -o easytools .
  ```
* Windows配布は単体`easytools.exe`を想定

---

## 13. サンプル `tools.yaml`

```yaml
api_key: devkey
tools:
  echo:
    group: Utility
    cmd: echo
    args: ["{{msg}}"]
    timeout: 5s
    max_stdout: 1048576
    max_stderr: 2097152
    allow_stdin: false
  jq-pretty:
    group: JSON
    cmd: jq
    args: ["."]
    allow_stdin: true
    timeout: 10s
    max_stdout: 1048576
    max_stderr: 2097152
```

---

## 14. 受け入れ基準（Acceptance Criteria）

* [ ] Homeで**2カラムフォーム**が表示され、値を変更→`Start Server`でサーバ起動、`Open Health`が動作
* [ ] 右1/3に**Test Run→Server Logs**が縦並びで表示
* [ ] Registryで**左2/3フォーム（スクロール）**＋**右1/3アコーディオン**が上2/3に、**Quick CMD**が下1/3に表示
* [ ] Toolsの**Add/Save/Delete/Import/Export**が動作し、Home/TestRun/QuickCMDのセレクトへ反映
* [ ] `/run`でparamsテンプレ展開・allow\_env反映・タイムアウト・出力上限が機能
* [ ] UI更新は**Fyneコールスレッド違反なし**

---

## 15. テスト観点（抜粋）

* **ユニット**:

  * `renderArgs`：未解決トークン、マルチ置換
  * `splitCSV/parseKV/joinKV`：空/空白/重複/順序
  * `cappedBuffer`：上限超過の切捨て
* **結合**:

  * `/run`：正常／コマンドなし／テンプレ未解決／allow\_env未許可／タイムアウト
  * YAML Import/Export：相互変換等価性
* **UI**:

  * サイドバー開閉、タブ切替、フォームスクロール、セレクト連動
  * goroutine→`fyne.Do` のUI反映タイミング

---

## 16. 将来拡張（Backlog）

* **多段グループ**（`Group/SubGroup`）や**タグ**でのフィルタリング
* **並列実行ログビュー**（テール表示・検索）
* **ヘルスダッシュボード**（全エンドポイントの可視化）
* **ロール/権限**（実行可否、編集権限の分離）
* **外部ストレージ連携**（設定のGit管理 等）

---

## 17. ライセンス / 表記（案）

* リポジトリ: `github.com/yashubustudio/easytools`
* ライセンス: **MIT**（予定）

---

## 18. 付録：主要UIのワイヤーフレーム（簡易）

```
┌──────────────────────────────────────────────────────────────────────┐
│ [≡] Server / API   Tools (Registry)                                  │ ← 左サイドバー（Collapse可）
├───┬───────────────────────────────────────────┬──────────────────────┤
│   │ Server / API (2/3)                         │  Test Run            │
│   │ ┌─────────────┬─────────────┐             │  [Tool v]            │
│   │ │ Addr/Base...│ Reload/Key..│             │  Params(JSON)        │
│   │ └─────────────┴─────────────┘             │  Env(JSON)           │
│   │ [Start] [Stop] [Open Health]   Status      │  Stdin               │
│   │                                           │  [POST /run]         │
│   │                                           │  Result              │
│   ├───────────────────────────────────────────┤──────────────────────┤
│   │                                           │  Server Logs         │
│   │                                           │  (auto update)       │
└───┴───────────────────────────────────────────┴──────────────────────┘
```

```
Registry:
┌──────────────────────────────────────────────────────────────────────┐
│ [≡] Server / API   Tools (Registry)                                  │
├──────────────────────────────────────────────────────────────────────┤
│ ┌───────────────────────────┬───────────────┐                        │
│ │ [Add][Save][Delete] ...   │ Tools by Group│                        │
│ │ Name / Group / Cmd / ...  │ (Accordion)   │  ← 上 2/3               │
│ │ (スクロール; Advanced常時表示)│               │                        │
│ └───────────────────────────┴───────────────┘                        │
│ Quick CMD (下 1/3): Tool / Params / Env / Stdin / [Quick Run] Result │
└──────────────────────────────────────────────────────────────────────┘
```

---
