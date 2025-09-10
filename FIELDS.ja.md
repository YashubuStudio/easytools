# モデルフィールドの日本語訳と説明

`internal/model/types.go` で定義されている構造体の各フィールドについて、日本語訳と機能を以下に示します。

## Tool

| フィールド | 日本語 | 説明 |
| --- | --- | --- |
| Group | グループ | ツールを分類するグループ名。 |
| Cmd | コマンド | 実行するコマンドのパス。 |
| Args | 引数 | コマンドに渡す引数。`{{name}}` のようなトークンは Params で置換されます。 |
| WorkDir | 作業ディレクトリ | コマンドを実行するディレクトリ。 |
| Env | 環境変数 | 追加で設定する環境変数のキーと値。 |
| AllowEnv | 環境変数許可リスト | リクエストから受け取る環境変数のホワイトリスト。 |
| Timeout | タイムアウト | 実行を中断するまでの時間。 |
| MaxStdout | 標準出力上限 | 取得する標準出力の最大バイト数。 |
| MaxStderr | 標準エラー上限 | 取得する標準エラーの最大バイト数。 |
| AllowStdin | 標準入力許可 | リクエストで標準入力を受け付けるかどうか。 |

## RunRequest

| フィールド | 日本語 | 説明 |
| --- | --- | --- |
| Tool | ツール名 | 実行するツールの名前。 |
| Params | パラメータ | Args 内のトークンを置換する値のマップ。 |
| Env | 環境変数 | 設定が許可したキーのみ環境変数として渡されます。 |
| Stdin | 標準入力 | AllowStdin が有効な場合に渡される標準入力文字列。 |

## RunResponse

| フィールド | 日本語 | 説明 |
| --- | --- | --- |
| Tool | ツール名 | 実行されたツールの名前。 |
| Command | 実行コマンド | 実行されたコマンドと引数の配列。 |
| ExitCode | 終了コード | プロセスの終了コード。 |
| Stdout | 標準出力 | 取得した標準出力。 |
| Stderr | 標準エラー | 取得した標準エラー。 |
| Duration | 実行時間 | 実行に要した時間（ミリ秒）。 |
| TimedOut | タイムアウト | 実行がタイムアウトしたかどうか。 |
| StartedAt | 開始時刻 | 実行開始時刻。 |
| EndedAt | 終了時刻 | 実行終了時刻。 |

## Paths

| フィールド | 日本語 | 説明 |
| --- | --- | --- |
| Run | 実行パス | `/run` エンドポイントのパス。 |
| Tools | ツール一覧パス | `/tools` エンドポイントのパス。 |
| Reload | 再読み込みパス | `/reload` エンドポイントのパス。 |
| Health | ヘルスチェックパス | `/health` エンドポイントのパス。 |

## ServerConfig

HTTP サーバー全体の動作を制御する設定です。

| フィールド | 日本語 | 説明 |
| --- | --- | --- |
| Addr | アドレス | サーバーが待ち受けるアドレス（例 `":8080"`）。 |
| BasePath | ベースパス | すべてのエンドポイントの先頭に付与されるパス。未指定時は `/v1`。 |
| APIKey | APIキー | リクエストヘッダ `X-API-Key` と照合する認証キー。空なら認証なし。 |
| CORS | CORS | `true` で `Access-Control-Allow-*` ヘッダを付与して `cors_origin` で指定したオリジンからの `GET`/`POST`/`OPTIONS` を許可。 |
| CORSOrigin | CORSオリジン | CORS 有効時に `Access-Control-Allow-Origin` に設定する値。未指定時は `*`。 |
| Tools | ツール | 利用可能なツール定義のマップ。キーがツール名。 |
| Paths | パス | `run` や `tools` など各 API エンドポイントのパス設定。 |

### CORS の詳細

CORS を無効 (`false`) にするとブラウザから別オリジン経由で呼び出した際に同一生成元ポリシーによりブロックされます。
有効 (`true`) にすると以下のヘッダが自動で付与され、プリフライト `OPTIONS` には `204 No Content` を返します。

- `Access-Control-Allow-Origin: <cors_origin の値または *>`
- `Access-Control-Allow-Headers: Content-Type, X-API-Key`
- `Access-Control-Allow-Methods: GET, POST, OPTIONS`

```yaml
ServerConfig:
  addr: ":8080"
  base_path: "/v1"
  api_key: "devkey"
  cors: true          # CORS を有効化
  cors_origin: "https://example.com"  # 許可オリジン（未指定時は *）
```

