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
