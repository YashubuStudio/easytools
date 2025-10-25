package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/yashubustudio/easytools/internal/appconfig"
	"github.com/yashubustudio/easytools/internal/gui"
	"github.com/yashubustudio/easytools/internal/server"
	"github.com/yashubustudio/easytools/internal/util"
	"github.com/yashubustudio/easytools/internal/webui"
)

func main() {
	var (
		serverFlag     = flag.Bool("server", false, "start the API server")
		webuiFlag      = flag.Bool("webui", false, "start the API server and launch the Web UI for configuration")
		configPath     = flag.String("config", appconfig.DefaultPath, "path to the YAML configuration file")
		addrOverride   = flag.String("addr", "", "override listen address (e.g. :8080 or 0.0.0.0:8080)")
		baseOverride   = flag.String("base-path", "", "override API base path")
		corsOverride   = flag.String("cors", "", "override CORS enable flag (true/false)")
		originOverride = flag.String("cors-origin", "", "override allowed CORS origin")
		listenAll      = flag.Bool("listen-all", false, "force listening on all interfaces (0.0.0.0)")
		webuiAddr      = flag.String("webui-addr", "127.0.0.1:18080", "address for the Web UI when --webui is set")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "easytools – GUI + CLI utility\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags]\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Examples:")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s --server\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  sudo %s --webui\n", os.Args[0])
	}
	flag.Parse()

	if handleHelp(flag.Args()) {
		return
	}

	if *webuiFlag {
		*serverFlag = true
	}

	if *serverFlag {
		if err := util.AttachConsole(); err != nil {
			log.Printf("warning: unable to attach console: %v", err)
		}
		if err := runServer(*configPath, *addrOverride, *baseOverride, *corsOverride, *originOverride, *listenAll, *webuiFlag, *webuiAddr); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if shouldUseGUI() {
		gui.Run()
		return
	}

	if err := util.AttachConsole(); err != nil {
		log.Printf("warning: unable to attach console: %v", err)
	}
	fmt.Println("No display detected – starting in CLI server mode. Use --help for options.")
	if err := runServer(*configPath, *addrOverride, *baseOverride, *corsOverride, *originOverride, *listenAll, *webuiFlag, *webuiAddr); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func runServer(configPath, addrOverride, baseOverride, corsOverride, originOverride string, listenAll bool, enableWebUI bool, webuiAddr string) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return err
	}
	if addrOverride != "" {
		cfg.Addr = addrOverride
	}
	if baseOverride != "" {
		cfg.BasePath = baseOverride
	}
	if corsOverride != "" {
		val, err := strconv.ParseBool(corsOverride)
		if err != nil {
			return fmt.Errorf("invalid --cors value: %w", err)
		}
		cfg.SetCORS(val)
	}
	if originOverride != "" {
		cfg.CORSOrigin = originOverride
	}
	if listenAll {
		cfg.SetListenAll(true)
	}
	if cfg.Tools == nil {
		cfg.Tools = appconfig.Default().Tools
	}
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := appconfig.Save(configPath, cfg); err != nil {
			return err
		}
	}

	srv := &server.LegacyServer{LogWriter: os.Stdout}
	if err := srv.Start(cfg.ToServerConfig()); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var webSrv *webui.Server
	if enableWebUI {
		webSrv = startWebUI(ctx, webuiAddr, configPath)
	}

	<-ctx.Done()
	util.Logf(os.Stdout, "[cli] shutting down...\n")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(); err != nil {
		util.Logf(os.Stdout, "[cli] server stop error: %v\n", err)
	}
	if webSrv != nil {
		_ = webSrv.Stop(shutdownCtx)
	}
	return nil
}

func startWebUI(ctx context.Context, addr, configPath string) *webui.Server {
	srv, errCh := webui.Start(ctx, addr, configPath, os.Stdout)
	go func() {
		if err, ok := <-errCh; ok && err != nil {
			util.Logf(os.Stdout, "[webui] error: %v\n", err)
		}
	}()
	return srv
}

func shouldUseGUI() bool {
	if v := strings.ToLower(os.Getenv("EASYTOOLS_FORCE_CLI")); v == "1" || v == "true" || v == "yes" {
		return false
	}
	if v := strings.ToLower(os.Getenv("FYNE_HEADLESS")); v == "1" || v == "true" {
		return false
	}
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		}
	}
	return true
}

type helpMode int

const (
	helpModeStandard helpMode = iota
	helpModeAI
	helpModeAdmin
)

func handleHelp(args []string) bool {
	if len(args) == 0 || !strings.EqualFold(args[0], "help") {
		return false
	}

	mode := helpModeStandard
	for _, arg := range args[1:] {
		switch strings.ToLower(arg) {
		case "-ai", "--ai":
			mode = helpModeAI
		case "-admin", "--admin":
			mode = helpModeAdmin
		}
	}

	printHelp(mode)
	return true
}

func printHelp(mode helpMode) {
	switch mode {
	case helpModeAI:
		printAIHelp()
	case helpModeAdmin:
		printAdminHelp()
	default:
		printStandardHelp()
	}
}

func printStandardHelp() {
	fmt.Println("EasyTools CLI コマンド一覧")
	fmt.Println()

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "コマンド\t説明")
	fmt.Fprintln(tw, "help\t標準ヘルプを表示します。")
	fmt.Fprintln(tw, "help -ai\tAI 向けの入出力例付きヘルプを表示します。")
	fmt.Fprintln(tw, "help -admin\t管理者向けヘルプを表示します。")
	fmt.Fprintln(tw, "--server\tCLI サーバーモードで起動します。")
	fmt.Fprintln(tw, "--webui\tサーバーを起動し Web UI をブラウザで開きます。")
	tw.Flush()

	fmt.Println()
	fmt.Println("主なフラグ")
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "%-12s\t%s\n", "--config <path>", "利用する YAML 設定ファイルを指定します。")
	fmt.Fprintf(tw, "%-12s\t%s\n", "--addr <host:port>", "リッスンするアドレスを上書きします。")
	fmt.Fprintf(tw, "%-12s\t%s\n", "--base-path <path>", "API のベースパスを変更します。")
	fmt.Fprintf(tw, "%-12s\t%s\n", "--cors", "CORS を有効 (true) / 無効 (false) にします。")
	fmt.Fprintf(tw, "%-12s\t%s\n", "--cors-origin <origin>", "許可するオリジンを設定します。")
	fmt.Fprintf(tw, "%-12s\t%s\n", "--listen-all", "0.0.0.0 で待ち受けるようにします。")
	fmt.Fprintf(tw, "%-12s\t%s\n", "--webui-addr <host:port>", "Web UI の待ち受けアドレスを指定します。")
	tw.Flush()

	fmt.Println()
	fmt.Println("使用例")
	fmt.Println("  easytools --server --config C:/tools.yaml --addr :8080")
	fmt.Println("  easytools --webui --config ./tools.yaml")
}

func printAIHelp() {
	fmt.Println("EasyTools CLI ヘルプ（AI 向け詳細版）")
	fmt.Println()
	fmt.Println("概要:")
	fmt.Println("  EasyTools はローカルで登録したコマンドを API サーバー経由で実行するツールです。")
	fmt.Println("  MCP プロトコルと REST API を提供し、tools.yaml に定義されたワークフローを呼び出します。")
	fmt.Println()
	fmt.Println("基本コマンド:")
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "入力\t説明")
	fmt.Fprintln(tw, "easytools help\t標準ヘルプ。")
	fmt.Fprintln(tw, "easytools help -admin\t管理者向けの全オプションを表示。")
	fmt.Fprintln(tw, "easytools --server --config tools.yaml\tCLI サーバーを設定ファイル付きで起動。")
	fmt.Fprintln(tw, "easytools --webui\tWeb UI を開きながらサーバーを起動。")
	tw.Flush()

	fmt.Println()
	fmt.Println("API 呼び出し例:")
	fmt.Println("  # MCP フォーマットで echo ツールを実行")
	fmt.Println("  curl -X POST http://localhost:8080/mcp/run \\")
	fmt.Println("    -H 'Content-Type: application/json' \\")
	fmt.Println("    -H 'X-API-Key: devkey' \\")
	fmt.Println("    -d '{\n          \"name\": \"echo\",\n          \"input\": {\n            \"params\": {\"msg\": \"hello\"}\n          }\n        }'")

	fmt.Println()
	fmt.Println("自然言語の説明:")
	fmt.Println("  1. `tools.yaml` に実行したいコマンドを登録します。")
	fmt.Println("  2. `easytools --server` または `easytools --webui` でサーバーを起動します。")
	fmt.Println("  3. API クライアントやエージェントから `/mcp/run` や `/run` にリクエストを送ります。")
	fmt.Println("  4. EasyTools が入力を検証し、登録済みコマンドを安全に実行して結果を返します。")

	fmt.Println()
	fmt.Println("出力例 (サーバーログ抜粋):")
	fmt.Println("  [cli] starting server on :8080")
	fmt.Println("  [cli] loaded 5 tools from tools.yaml")
}

func printAdminHelp() {
	fmt.Println("EasyTools CLI 管理者向けヘルプ")
	fmt.Println()
	fmt.Println("起動コマンド:")
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "コマンド\t説明")
	fmt.Fprintln(tw, "easytools --server\tGUI なしでサーバーを起動し、標準出力にログを出します。")
	fmt.Fprintln(tw, "easytools --webui\tサーバーを起動し、ブラウザベースの設定 UI を立ち上げます。")
	fmt.Fprintln(tw, "easytools\tGUI が利用可能なら管理画面を開き、なければ CLI サーバーモードにフォールバックします。")
	tw.Flush()

	fmt.Println()
	fmt.Println("サーバーフラグ詳細:")
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "フラグ\t説明")
	fmt.Fprintln(tw, "--config <path>\tツール定義とサーバー設定を含む YAML ファイル。存在しない場合は雛形を生成。")
	fmt.Fprintln(tw, "--addr <host:port>\tHTTP サーバーのリッスンアドレス。例: :8080, 0.0.0.0:8080")
	fmt.Fprintln(tw, "--base-path <path>\tAPI のベースパス (例: /easytools)。")
	fmt.Fprintln(tw, "--cors <true|false>\tCORS ヘッダの有効 / 無効を切り替え。")
	fmt.Fprintln(tw, "--cors-origin <origin>\t`Access-Control-Allow-Origin` に設定する値。")
	fmt.Fprintln(tw, "--listen-all\t0.0.0.0 で待ち受けるショートカット。LAN 配信時に利用。")
	fmt.Fprintln(tw, "--webui-addr <host:port>\tWeb UI (ブラウザ) 用のローカルリッスンアドレス。デフォルト 127.0.0.1:18080。")
	tw.Flush()

	fmt.Println()
	fmt.Println("運用メモ:")
	fmt.Println("  • 環境変数 `EASYTOOLS_FORCE_CLI=1` で GUI を無効化し CLI 起動を強制できます。")
	fmt.Println("  • `FYNE_HEADLESS=1` も GUI 無効化に利用可能です (コンテナ運用向け)。")
	fmt.Println("  • SSH セッションでは自動的に CLI モードに切り替わります。")
	fmt.Println("  • `POST /reload` を叩くと稼働中サーバーが tools.yaml を再読み込みします。")
	fmt.Println("  • MCP パッケージは `GET /mcp/package`、ヘルスチェックは `GET /healthz` で確認できます。")

	fmt.Println()
	fmt.Println("設定ファイルの雛形:")
	fmt.Println("  tools.yaml に含まれる代表的なキー:")
	fmt.Println("    tools.<name>.cmd        実行するバイナリ")
	fmt.Println("    tools.<name>.args       固定引数の配列")
	fmt.Println("    tools.<name>.timeout    タイムアウト (例: 30s)")
	fmt.Println("    tools.<name>.env        追加する環境変数マップ")
	fmt.Println("    tools.<name>.limits     実行時間や I/O サイズ制限")
}
