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
| `POST` | `/mcp/run` | MCP 仕様の JSON 入力を検証し、対応する登録 API をサンドボックスで実行します。互換性のため従来の `/run` 形式も受け付けます。|
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

#### `/mcp/run` のリクエスト仕様

- **HTTP メソッド / パス**: `POST /mcp/run`（`base_path` や `paths.mcp_invoke` の設定がある場合はそれに従います）
- **ヘッダー**: `Content-Type: application/json` は必須。API キー認証を有効にしているときは `X-API-Key: <設定した値>` を追加してください。
- **リクエストボディ**: `InvokeRequest` 形式の JSON を 1 件送信します。

```json
{
  "name": "<tools.yaml で定義したツール名>",
  "input": {
    "params": {"<テンプレート変数>": "値", ...},
    "env": {"<許可された環境変数>": "値", ...},
    "stdin": "ツールに渡す標準入力"
  }
}
```

| フィールド | 必須 | 説明 |
| --- | --- | --- |
| `name` | ✅ | `tools.yaml` のキー名。前後の空白は取り除かれ、空文字列の場合は 400 が返ります。|
| `input` | 任意 | 追加の入力をまとめたオブジェクト。省略時は空の入力として扱われます。|
| `input.params` | 条件付き | コマンドライン引数テンプレート `{{token}}` を埋める値。ツールの `input.params` で `required: true` の項目、またはテンプレートから自動検出されたトークンはすべて必須です。|
| `input.env` | 任意 | `allow_env` に含まれる環境変数名、もしくは `input.env` で明示した項目に値を渡します。許可されていないキーを送信すると 400 になります。|
| `input.stdin` | 条件付き | `allow_stdin: true` かつ `input.stdin.required: true` のツールでは必須です。禁止されている状態で送ると 400 になります。|

##### テンプレートの展開ルール

- `args` に含まれる `{{token}}` 形式のトークンが自動検出され、`input.params[token]` で指定した値に置き換わります。
- 置換は文字列置換で行われ、未解決の `{{` が残っていると `arg template error` として 400 が返ります。
- `input.params` を設定していないツールでもテンプレートから検出したトークンは必須です。
- 値は内部的に `fmt.Sprint` で文字列化され、同じトークンが複数回現れても 1 度指定すればすべて置換されます。

##### レスポンスとステータスコード

- 成功時は `200 OK` で `{"name", "success", "output"}` を含む JSON を返します。`output.command` には実際に実行したバイナリと引数が格納されます。
- コマンドが非ゼロ終了、タイムアウトなどで失敗した場合は本文を返しつつも HTTP ステータスは `400 Bad Request` になります。
- バリデーションエラーや未登録ツールは `400 Bad Request` または `404 Not Found`、API キー不一致は `401 Unauthorized`、未対応メソッドは `405 Method Not Allowed` を返します。

##### 認証 (`X-API-Key`)

- サーバー設定で `api_key` を指定すると全エンドポイントで `X-API-Key` ヘッダーが必須になります。
- 無効または欠落している場合は `401 Unauthorized` と `{"error": "missing/invalid api key"}` を返します。
- `cors: true` の場合は `Access-Control-Allow-Headers: Content-Type, X-API-Key` を含むレスポンスヘッダーと、プリフライト要求に対する `204 No Content` を返します。

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
