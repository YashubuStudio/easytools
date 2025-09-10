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

```bash
go mod tidy
go build -o easytools.exe ./cmd/legacy-exec-gui
```

## 実行

```bash
./easytools
```

バイナリを起動すると GUI が開きます。サーバーアドレスやパスを設定し、ツールを登録して **Start Server** をクリックしてください。実行中は以下のエンドポイントが利用できます（デフォルト値を表示）。

| Method | Path          | Description           |
|--------|---------------|-----------------------|
| GET    | `/v1/healthz` | ヘルスチェック        |
| GET    | `/v1/tools`   | 登録済みツール一覧    |
| POST   | `/v1/run`     | ツールの実行          |
| POST   | `/v1/reload`  | 設定の再読み込み      |

リクエスト例:

```bash
curl -X POST 'http://localhost:8080/v1/run' \
  -H 'X-API-Key: devkey' \
  -d '{"tool":"echo","params":{"msg":"hello"}}'
```

## アプリケーションウィンドウ

GUI はサーバー管理とツール登録の 2 つのタブで構成されています。

### Server / API

- **サーバー設定** – 左側の入力欄でアドレスやベースパス、各エンドポイント、API キー、CORS の有効/無効を設定します。**Start Server**/**Stop Server** ボタンで HTTP サーバーを制御し、上部に状態が表示されます。
- **テストコンソール** – 右側のパネルでツールを選択し、JSON 形式のパラメータや環境変数を入力して `/run` へテストリクエストを送ります。結果はボタン下に表示されます。
- **サーバーログ** – 下部の領域でリアルタイムのログを確認できます。

### Tools (Registry)

- **ツール入力フォーム** – 左 1/3 のフォームでツールの登録・編集を行います。Name、Group、Cmd（実行ファイル）、Args（カンマ区切り。`{{msg}}` のようなトークンは Params で置換）、Workdir、Env、AllowEnv、Timeout、MaxStdout、MaxStderr、Stdin を入力し、Add/Save/Delete や YAML のインポート/エクスポートが利用できます。
- **ツール一覧** – 中央のアコーディオンで Group ごとにツールが表示され、簡単に選択できます。
- **Quick CMD** – 右 1/3 のパネルで選択したツールを即座に実行できます。JSON 形式のパラメータ・環境変数・stdin を入力すると HTTP レスポンスが表示され、ツールの動作確認に役立ちます。
  - `Params (JSON)` に入力した値は Args 内の `{{名前}}` トークンを置換します。
  - `Env (JSON)` は AllowEnv に指定されたキーのみ環境変数として適用されます。
  - `Stdin` は Allow Stdin が有効な場合に標準入力へ渡されます。
  - 例: `Cmd: /usr/bin/echo`, `Args: ["{{msg}}"]` のツールで `Params: {"msg":"hello"}` を入力すると `/usr/bin/echo hello` が実行されます。

## ライセンス
MIT

