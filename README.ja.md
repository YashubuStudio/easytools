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

## ライセンス
MIT

