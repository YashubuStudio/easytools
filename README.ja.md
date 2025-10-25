# EasyTools

*English version: [README.md](README.md)*

EasyTools はエージェントからの呼び出しを Model Context Protocol (MCP) として受け付け、サンドボックス化したコマンド実行を提供する軽量フレームワークです。JSON 形式で渡された入力から事前に定義されたフィールドのみを抽出し、登録済み API と突き合わせて安全にコマンドを起動します。レスポンスは検証とマスク処理を経て返送されるため、秘匿情報を含むワークフローでも安心して利用できます。

## 特長
- **MCP エンドポイント** – `/mcp/run` で実行、`/mcp/package` で登録 API 記述子を配信。名称・引数・制約・リクエスト/レスポンス例・自然言語による解説を含むディスクリプタを提供します。
- **セキュアなコマンド実行** – 未登録コマンド、シェル解釈（パイプ・リダイレクト）、`sudo`、任意プロセス起動はブロック。一般ユーザ権限で実行し、時間・メモリ・入出力パスを制限します。
- **登録 API ごとの詳細設定** – ワーキングディレクトリ、コマンド名、タイムアウト、環境変数などを CLI から細かく設定可能。エージェント向けの入力検証とマスク機構を備えています。
- **ローカルネットワーク配信対応** – CORS と API キー認証を設定でき、LAN 内の複数クライアントから安全に呼び出せます。
- **軽量な Go 製実装** – 依存関係を最小化し、各 OS 向けバイナリを用意。再現性確保のため実装・設定手順、API スキーマ、実験スクリプトをリポジトリに公開し、コミット ID を固定して検証可能です。

## ビルド
Go 1.24.5 と Fyne GUI ツールキットが必要です。

```bash
go mod tidy
go build -o easytools ./cmd/easytools
```

## CLI とサーバーの起動

```bash
./easytools --server \
  --config /path/to/tools.yaml \
  --addr :8080
```

GUI が利用できる環境ではダブルクリックまたはフラグなし起動で管理画面が開きます。GUI/CLI どちらでも `tools.yaml` を読み書きし、コマンドライン引数が優先されます。

CLI では登録 API ごとに以下を設定できます:

- `cmd` / `args`: 実行するコマンドと固定引数（シェル解釈は無効）
- `workdir`: 作業ディレクトリ
- `timeout`: タイムアウト（ミリ秒）
- `env` / `allow_env`: 事前設定とホワイトリスト型の環境変数
- `stdin`: 標準入力の許可有無と初期値
- `limits`: 実行時間・メモリ・入出力サイズの上限

## HTTP エンドポイント

| Method | Path | 説明 |
| --- | --- | --- |
| `POST` | `/mcp/run` | MCP 仕様の JSON 入力を検証し、対応する登録 API をサンドボックスで実行します。|
| `GET` | `/mcp/package` | 利用可能な API のディスクリプタ一覧（名称、引数、制約、例、自然言語解説）を返します。|
| `POST` | `/run` | MCP 以外のシンプルな JSON リクエストでコマンドを実行します。|
| `GET` | `/tools` | 登録済みツール一覧を返します。|
| `POST` | `/reload` | `tools.yaml` を再読み込みします。|
| `GET` | `/healthz` | サーバーのヘルスチェック。|

すべてのエンドポイントは `base_path` と `paths.*` の設定で変更可能です。API キー認証は `X-API-Key` ヘッダで行い、CORS を有効にすると `Access-Control-Allow-*` ヘッダが追加されます。

### MCP リクエスト例

```bash
curl -X POST 'http://localhost:8080/mcp/run' \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: devkey' \
  -d '{
        "name": "echo",
        "input": {
          "params": {"msg": "hello"}
        }
      }'
```

> [!TIP]
> Windows の `cmd.exe` ではシングルクォートとバックスラッシュによる改行継続が利用できません。
> 代わりにダブルクォートとキャレット (`^`) を使ってください。
>
> ```cmd
> curl -X POST "http://localhost:8080/mcp/run" ^
>   -H "Content-Type: application/json" ^
>   -H "X-API-Key: devkey" ^
>   -d "{\\
>         \"name\": \"echo\",\\
>         \"input\": {\\
>           \"params\": {\"msg\": \"hello\"}\\
>         }\\
>       }"
> ```
>
> PowerShell ではバックスラッシュの代わりにバッククォート (`\``) の行継続を使えますが、シングルクォートのまま実行してください。

レスポンスは検証後の JSON を 1 件返し、マスク対象の値は自動的に伏字化されます。

### MCP パッケージ例

```bash
curl -H 'X-API-Key: devkey' \
  http://localhost:8080/mcp/package
```

```cmd
curl -H "X-API-Key: devkey" http://localhost:8080/mcp/package
```

結果には API 名称、引数定義、制約、サンプルリクエスト/レスポンス、自然言語解説（任意）が含まれます。

### シンプルな JSON リクエスト例

```bash
curl -X POST 'http://localhost:8080/run' \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: devkey' \
  -d '{
        "tool": "echo",
        "params": {"msg": "hello"}
      }'
```

レスポンスには実行コマンド、標準出力・標準エラー、終了コード、時間情報を含む `RunResponse` が返ります。

### `git pull` コマンドを追加する手順

1. 初回起動時に生成される `tools.yaml` を開き、`tools` マップに以下のエントリを追記します。

   ```yaml
   tools:
     git_pull:
       cmd: git
       args: ["pull"]
       workdir: /path/to/your/repository
       timeout: 30s
   ```

   `workdir` は更新したいリポジトリのパスに置き換えてください。必要に応じて `env`、`allow_env`、`stdin`、`limits` などのキーも設定できます。

2. GUI から再読み込みするか、`POST http://localhost:8080/reload` に `X-API-Key` ヘッダを付けて送信し、サーバーに設定を反映させます。

3. 追加したツールを実行します。

   ```bash
   curl -X POST 'http://localhost:8080/run' \
     -H 'Content-Type: application/json' \
     -H 'X-API-Key: devkey' \
     -d '{"tool": "git_pull"}'
   ```

   MCP から利用する場合は、ペイロード内の `"name"` に `git_pull` を指定します。

## 安全性とサンドボックス
- 実行するプロセスは一般ユーザ権限のみ。`sudo` や任意 UID の昇格は不可。
- シェル構文（パイプ・リダイレクト・サブシェルなど）は解釈せず、登録済みバイナリのみを直接起動。
- リソース上限（時間・メモリ・標準入出力サイズ）を設定し、制限値を超えると強制終了します。
- 入出力は事前に登録されたパスのみ許可され、サンドボックス外への書き込みを防ぎます。

## 再現性の確保
ビルド手順、設定例、API 記述子スキーマ、評価用スクリプトはリポジトリに同梱されています。ドキュメントでは検証に使用したコミット ID を固定し、移植性と軽量性を重視した Go 実装を各 OS 向けバイナリとして提供します。

## ライセンス
詳細は [LICENSE](LICENSE) を参照してください。
