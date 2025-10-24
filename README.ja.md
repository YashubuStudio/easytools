# EasyTools

*English version: [README.md](README.md)*

EasyTools は既存のコマンドラインプログラムをラップして HTTP API として公開するツールです。デスクトップGUIを備えており、ツールの登録・編集・テストを行いながら組み込みHTTPサーバーを制御できます。設定は YAML としてインポート・エクスポート可能です。

## 機能
- 既存のスクリプトやバイナリを登録し、設定可能な HTTP エンドポイントとして公開
- デスクトップGUI (Fyne) でツールの追加/編集、サーバーの開始/停止、簡易テスト
- ツール毎にタイムアウト、環境変数ホワイトリスト、stdout/stderr サイズ上限、任意の stdin などの安全機能
- ログビューアとテストコンソールを内蔵し即時フィードバック
- 設定を `tools.yaml` としてインポート/エクスポート

## ビルド
Go 1.24.5 と Fyne GUI ツールキットが必要です。

### Windows
```bash
go mod tidy
go build -o easytools ./cmd/easytools
```

### Linux
```bash
go mod tidy
go build -o easytools ./cmd/easytools
```

## 実行

```bash
./easytools
```

フラグなしで起動すると（デスクトップではダブルクリックで）GUI が開きます。`DISPLAY` が無い SSH 環境などヘッドレス環境では自動的に CLI モードへフォールバックし、GUI の代わりに HTTP サーバーを起動します。手動で制御したい場合は以下のフラグを利用してください。

```bash
./easytools --server           # HTTP サーバーのみ起動
sudo ./easytools --webui       # サーバーを起動し、127.0.0.1:18080 に設定用 Web UI を開く
./easytools --config /path/to/tools.yaml --addr :9090
```

CLI は GUI と同じ `tools.yaml` を読み書きし、コマンドラインで指定した値を優先します。`--webui` で有効になる Web UI は YAML を編集するための簡易設定画面で、管理者権限での利用を想定しています。

GUI または CLI モードでサーバーアドレスやパスを設定し、ツールを登録して **Start Server** をクリックしてください。実行中は以下のエンドポイントが利用できます（デフォルト値を表示）。

| Method | Path                     | Description                         |
|--------|--------------------------|-------------------------------------|
| GET    | `/v1/healthz`            | ヘルスチェック                      |
| GET    | `/v1/tools`              | 登録済みツール一覧                  |
| GET    | `/v1/tools/{group}/{name}` | ツールの実行 (API Key なし)       |
| POST   | `/v1/run`                | ツールの実行                        |
| POST   | `/v1/reload`             | 設定の再読み込み                    |
| GET    | `/v1/mcp/package`        | 登録ツールを MCP パッケージ形式で取得 |
| POST   | `/v1/mcp/run`            | MCP 形式の入力でツールを実行           |

各エンドポイントのパスはサーバー設定の `paths` セクションで変更できます。デフォルト値と役割は以下の通りです:

- **`base_path`** (`/v1`): すべてのエンドポイントに付与されるプレフィックス。
- **`paths.tools`** (`/tools`): `GET` で登録済みツール一覧。`GET /{group}/{name}` は API キー未設定時に単一ツールを実行。
- **`paths.run`** (`/run`): `POST` の JSON 本文で指定されたツールを実行。
- **`paths.reload`** (`/reload`): `POST` で `tools.yaml` を再読み込み。
- **`paths.health`** (`/healthz`): `GET` でサーバー状態を返すヘルスチェック。
- **`paths.mcp_package`** (`/mcp/package`): `GET` で登録ツールを MCP パッケージとして返すマニフェスト。
- **`paths.mcp_invoke`** (`/mcp/run`): `POST` で MCP 入力形式の一括リクエストを受け取り、単一 JSON レスポンスを返します。

リクエスト例:

```bash
curl -X POST 'http://localhost:8080/v1/run' \
  -H 'X-API-Key: devkey' \
  -d '{"tool":"echo","params":{"msg":"hello"}}'
```

MCP リクエストとレスポンスの例:

```bash
curl -X POST 'http://localhost:8080/v1/mcp/run' \
  -H 'X-API-Key: devkey' \
  -d '{
        "name": "echo",
        "input": {
          "params": {"msg": "hello"}
        }
      }'
```

レスポンスは次の 1 つの JSON にまとまります:

```json
{
  "name": "echo",
  "success": true,
  "output": {
    "command": ["/usr/bin/echo", "hello"],
    "exit_code": 0,
    "stdout": "hello\n",
    "stderr": "",
    "duration_ms": 5,
    "timed_out": false,
    "started_at": "2025-09-10T07:42:22Z",
    "ended_at": "2025-09-10T07:42:22Z"
  }
}
```

MCP パッケージのマニフェストは以下で取得できます:

```bash
curl -X GET 'http://localhost:8080/v1/mcp/package' -H 'X-API-Key: devkey'
```

API キーを設定していない場合は、次のように直接ツールを呼び出せます:

```bash
curl http://localhost:8080/v1/tools/echo
```

## アプリケーションウィンドウ

GUI はサーバー管理とツール登録の 2 つのタブで構成されています。

### Server / API

- **サーバー設定** – 左側の入力欄でアドレスやベースパス、各エンドポイント、API キー、CORS の有効/無効とオリジンを設定します。**Start Server**/**Stop Server** ボタンで HTTP サーバーを制御し、上部に状態が表示されます。
- **テストコンソール** – 右側のパネルでツールを選択し、JSON 形式のパラメータや環境変数を入力して `/run` へテストリクエストを送ります。結果はボタン下に表示されます。
- **サーバーログ** – 下部の領域でリアルタイムのログを確認できます。

### Tools (Registry)

- **ツール入力フォーム** – 左 1/3 のフォームでツールの登録・編集を行います。Name、Group、Cmd（実行ファイル）、Args（カンマ区切り。`{{msg}}` のようなトークンは Params で置換）、Workdir、Env、AllowEnv、Timeout、MaxStdout、MaxStderr、Stdin を入力し、Add/Save/Delete や YAML のインポート/エクスポートが利用できます。変更は自動的に `tools.yaml` に保存されます。
- **作業ディレクトリ** – Workdir にコマンドを実行するディレクトリを指定できます。`git` のようにディレクトリに依存するコマンドはここにリポジトリのパスなどを設定してください。未指定の場合は EasyTools の起動ディレクトリで実行されます。
- **ツール一覧** – 中央のアコーディオンで Group ごとにツールが表示され、簡単に選択できます。
- **Quick CMD** – 右 1/3 のパネルで選択したツールを即座に実行できます。JSON 形式のパラメータ・環境変数・stdin を入力すると HTTP レスポンスが表示され、ツールの動作確認に役立ちます。
  - `Params (JSON)` に入力した値は Args 内の `{{名前}}` トークンを置換します。
  - `Env (JSON)` は AllowEnv に指定されたキーのみ環境変数として適用されます。
  - `Stdin` は Allow Stdin が有効な場合に標準入力へ渡されます。
  - 例: `Cmd: /usr/bin/echo`, `Args: ["{{msg}}"]` のツールで `Params: {"msg":"hello"}` を入力すると `/usr/bin/echo hello` が実行されます。

### git コマンドの例

GitHub ユーザーに馴染みのある `git status` を HTTP API として公開する設定例です。

```yaml
tools:
  repo-status:
    cmd: git
    args: ["status", "--short"]
    workdir: /path/to/repository
```

GUI の **Quick CMD** で `repo-status` を選び、`/run` へリクエストを送るとリポジトリの状態を取得できます。

## ライセンス
個人利用は自由（無償）で行えますが、使用に伴う責任はすべて利用者にあります。商用利用をご希望の際は事前にご連絡ください。バグやエラーなどがありましたら、お気軽にコメントをお寄せください。詳細は [LICENSE](LICENSE) をご覧ください。

