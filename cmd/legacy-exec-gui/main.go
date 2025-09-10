package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"image/color"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gopkg.in/yaml.v3"

	"github.com/yashubustudio/easytools/internal/logging"
	"github.com/yashubustudio/easytools/internal/model"
	"github.com/yashubustudio/easytools/internal/server"
	"github.com/yashubustudio/easytools/internal/util"
)

func main() {
	// ログはGUIのテキストエリアにも出す
	ring := logging.NewLogRing(2000)
	log.SetOutput(io.MultiWriter(os.Stdout, ring))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	a := app.NewWithID("legacy.exec.gui")
	w := a.NewWindow("Legacy Exec – Manager")

	// 既定値
	defAddr := a.Preferences().StringWithFallback("addr", ":8080")
	defBase := a.Preferences().StringWithFallback("base", "/v1")
	defRun := a.Preferences().StringWithFallback("path.run", "/run")
	defTools := a.Preferences().StringWithFallback("path.tools", "/tools")
	defReload := a.Preferences().StringWithFallback("path.reload", "/reload")
	defHealth := a.Preferences().StringWithFallback("path.health", "/healthz")
	defKey := a.Preferences().StringWithFallback("api_key", "devkey")
	defCORS := a.Preferences().BoolWithFallback("cors", true)

	// サーバ設定 Entry
	addrEntry := widget.NewEntry()
	addrEntry.SetText(defAddr)
	baseEntry := widget.NewEntry()
	baseEntry.SetText(defBase)
	runEntry := widget.NewEntry()
	runEntry.SetText(defRun)
	toolsEntry := widget.NewEntry()
	toolsEntry.SetText(defTools)
	reloadEntry := widget.NewEntry()
	reloadEntry.SetText(defReload)
	healthEntry := widget.NewEntry()
	healthEntry.SetText(defHealth)
	keyEntry := widget.NewPasswordEntry()
	keyEntry.SetText(defKey)
	corsCheck := widget.NewCheck("Enable CORS", nil)
	corsCheck.SetChecked(defCORS)

	// 状態
	var srv server.LegacyServer
	srv.LogWriter = ring
	statusLbl := widget.NewLabel("Server: stopped")

	// ツール（ロード）
	tools := map[string]model.Tool{
		"echo": {Cmd: "echo", Args: []string{"{{msg}}"}, Timeout: "5s", MaxStdout: 1 << 20},
	}
	if b, err := os.ReadFile("tools.yaml"); err == nil {
		var tmp struct {
			APIKey string                `yaml:"api_key"`
			Tools  map[string]model.Tool `yaml:"tools"`
		}
		if yaml.Unmarshal(b, &tmp) == nil && len(tmp.Tools) > 0 {
			tools = tmp.Tools
			if tmp.APIKey != "" {
				keyEntry.SetText(tmp.APIKey)
			}
		}
	}
	toolNames := func() []string {
		ns := make([]string, 0, len(tools))
		for k := range tools {
			ns = append(ns, k)
		}
		sort.Strings(ns)
		return ns
	}

	// 前方宣言
	var selected string
	var loadTool func(name string)
	var refreshSelectors func()
	var buildAccordion func()
	var runSelect *widget.Select
	var quickSelect *widget.Select

	// Registry入力
	groupEntry := widget.NewEntry()
	nameEntry := widget.NewEntry()
	cmdEntry := widget.NewEntry()
	argsEntry := widget.NewEntry() // comma
	workdirEntry := widget.NewEntry()
	envEntry := widget.NewMultiLineEntry() // KEY=VAL per line
	allowEnvEntry := widget.NewEntry()     // comma
	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText("30s")
	maxOutEntry := widget.NewEntry()
	maxOutEntry.SetText("1048576")
	maxErrEntry := widget.NewEntry()
	maxErrEntry.SetText("2097152")
	stdinCheck := widget.NewCheck("Allow stdin", nil)

	// 余白ユーティリティ
	withMargin := func(o fyne.CanvasObject) fyne.CanvasObject {
		m := float32(theme.TextSize())
		padH := func() fyne.CanvasObject {
			r := canvas.NewRectangle(color.Transparent)
			r.SetMinSize(fyne.NewSize(0, m))
			return r
		}
		padV := func() fyne.CanvasObject {
			r := canvas.NewRectangle(color.Transparent)
			r.SetMinSize(fyne.NewSize(m, 0))
			return r
		}
		return container.NewBorder(padH(), padH(), padV(), padV(), o)
	}

	// ▼ ラベル行
	// main と (note) を同一行・横並びにし、note は小さいフォントで 1 行固定。
	// 注釈の実測幅を加味して左カラム幅を伸ばし、縦に潰れたり 1 文字縦並びになるのを防ぐ。
	// ▼ ラベル行（注釈は \n で分割し、行ごとに canvas.Text を行間ゼロで配置）
	formRow := func(main, note string, w fyne.CanvasObject, baseLeftWidth float32) fyne.CanvasObject {
		mainLbl := widget.NewLabel(main)

		var leftContent fyne.CanvasObject
		if strings.TrimSpace(note) != "" {
			// 小さめの文字サイズ（float32 で扱う）
			sz := theme.TextSize() - 3
			if sz < float32(8) {
				sz = float32(8)
			}

			// 行高は計測値を使用（見た目が詰まるように少しだけマイナス補正してもOK）
			lineHeight := fyne.MeasureText("Ag", sz, fyne.TextStyle{}).Height

			// ラベルと注釈を行間ゼロで手動配置
			leftBox := container.NewWithoutLayout()
			mainLbl.Move(fyne.NewPos(0, 0))
			leftBox.Add(mainLbl)

			startY := mainLbl.MinSize().Height // ラベル直下から開始
			lines := strings.Split(note, "\n")
			for i, line := range lines {
				t := canvas.NewText(line, theme.ForegroundColor())
				t.TextSize = sz
				t.Move(fyne.NewPos(0, startY+float32(i)*lineHeight))
				leftBox.Add(t)
			}

			// ラベル高さ + 行数分の高さでラッパのサイズを決める
			totalH := startY + float32(len(lines))*lineHeight
			// GridWrap が高さを認識できるように明示的な最小サイズの箱で包む
			leftContent = container.NewGridWrap(
				fyne.NewSize(baseLeftWidth, totalH),
				leftBox,
			)
		} else {
			leftContent = container.NewGridWrap(
				fyne.NewSize(baseLeftWidth, mainLbl.MinSize().Height),
				mainLbl,
			)
		}

		// 右側が入力ウィジェット
		return container.NewBorder(nil, nil, leftContent, nil, w)
	}

	loadTool = func(name string) {
		t := tools[name]
		groupEntry.SetText(t.Group)
		nameEntry.SetText(name)
		cmdEntry.SetText(t.Cmd)
		argsEntry.SetText(strings.Join(t.Args, ","))
		workdirEntry.SetText(t.WorkDir)
		envEntry.SetText(util.JoinKV(t.Env))
		allowEnvEntry.SetText(strings.Join(t.AllowEnv, ","))
		if t.Timeout != "" {
			timeoutEntry.SetText(t.Timeout)
		} else {
			timeoutEntry.SetText("30s")
		}
		if t.MaxStdout > 0 {
			maxOutEntry.SetText(fmt.Sprint(t.MaxStdout))
		} else {
			maxOutEntry.SetText("1048576")
		}
		if t.MaxStderr > 0 {
			maxErrEntry.SetText(fmt.Sprint(t.MaxStderr))
		} else {
			maxErrEntry.SetText("2097152")
		}
		stdinCheck.SetChecked(t.AllowStdin)
	}

	refreshSelectors = func() {
		if runSelect != nil {
			runSelect.Options = toolNames()
			runSelect.Refresh()
		}
		if quickSelect != nil {
			quickSelect.Options = toolNames()
			quickSelect.Refresh()
		}
	}

	addTemplate := func() {
		groupEntry.SetText("")
		nameEntry.SetText("new_tool")
		cmdEntry.SetText("/usr/bin/echo")
		argsEntry.SetText("{{msg}}")
		workdirEntry.SetText("")
		envEntry.SetText("")
		allowEnvEntry.SetText("")
		timeoutEntry.SetText("30s")
		maxOutEntry.SetText("1048576")
		maxErrEntry.SetText("2097152")
		stdinCheck.SetChecked(false)
	}
	importYAML := func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			var y struct {
				APIKey string                `yaml:"api_key"`
				Tools  map[string]model.Tool `yaml:"tools"`
			}
			b, _ := io.ReadAll(r)
			if err := yaml.Unmarshal(b, &y); err != nil {
				dialog.ShowError(err, w)
				return
			}
			if y.Tools != nil {
				tools = y.Tools
				selected = ""
				buildAccordion()
				refreshSelectors()
			}
			if y.APIKey != "" {
				keyEntry.SetText(y.APIKey)
			}
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
		fd.Show()
	}
	exportYAML := func() {
		sd := dialog.NewFileSave(func(r fyne.URIWriteCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			y := struct {
				APIKey string                `yaml:"api_key"`
				Tools  map[string]model.Tool `yaml:"tools"`
			}{APIKey: keyEntry.Text, Tools: tools}
			b, err := yaml.Marshal(y)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			_, _ = r.Write(b)
		}, w)
		sd.SetFileName("tools.yaml")
		sd.Show()
	}

	saveTool := func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			dialog.ShowInformation("Save", "name is required", w)
			return
		}
		t := model.Tool{
			Group:      strings.TrimSpace(groupEntry.Text),
			Cmd:        strings.TrimSpace(cmdEntry.Text),
			Args:       util.SplitCSV(argsEntry.Text),
			WorkDir:    strings.TrimSpace(workdirEntry.Text),
			Env:        util.ParseKV(envEntry.Text),
			AllowEnv:   util.SplitCSV(allowEnvEntry.Text),
			Timeout:    strings.TrimSpace(timeoutEntry.Text),
			MaxStdout:  util.AtoiOr(maxOutEntry.Text, 1<<20),
			MaxStderr:  util.AtoiOr(maxErrEntry.Text, 2<<20),
			AllowStdin: stdinCheck.Checked,
		}
		if t.Cmd == "" {
			dialog.ShowInformation("Save", "cmd is required", w)
			return
		}
		if selected != "" && selected != name {
			delete(tools, selected)
		}
		tools[name] = t
		selected = name
		buildAccordion()
		refreshSelectors()
	}

	delTool := func() {
		if selected == "" {
			return
		}
		delete(tools, selected)
		selected = ""
		buildAccordion()
		refreshSelectors()
		groupEntry.SetText("")
		nameEntry.SetText("")
		cmdEntry.SetText("")
		argsEntry.SetText("")
		workdirEntry.SetText("")
		envEntry.SetText("")
		allowEnvEntry.SetText("")
		timeoutEntry.SetText("30s")
		stdinCheck.SetChecked(false)
	}

	accHolder := container.NewMax()
	buildAccordion = func() {
		grouped := map[string][]string{}
		for name, t := range tools {
			g := strings.TrimSpace(t.Group)
			if g == "" {
				g = "Ungrouped"
			}
			grouped[g] = append(grouped[g], name)
		}
		gs := make([]string, 0, len(grouped))
		for g := range grouped {
			gs = append(gs, g)
		}
		sort.Strings(gs)

		acc := widget.NewAccordion()
		for _, g := range gs {
			names := grouped[g]
			sort.Strings(names)
			listNames := append([]string(nil), names...)
			lst := widget.NewList(
				func() int { return len(listNames) },
				func() fyne.CanvasObject { return widget.NewLabel("tool") },
				func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(listNames[i]) },
			)
			lst.OnSelected = func(i widget.ListItemID) {
				if i < 0 || i >= len(listNames) {
					return
				}
				selected = listNames[i]
				loadTool(selected)
			}
			acc.Append(widget.NewAccordionItem(g, lst))
		}
		accHolder.Objects = []fyne.CanvasObject{acc}
		accHolder.Refresh()
	}
	buildAccordion()

	// Home: Test Run + Logs
	runSelect = widget.NewSelect(toolNames(), nil)
	paramsEntryRun := widget.NewMultiLineEntry()
	paramsEntryRun.SetPlaceHolder(`{"msg":"hello"}`)
	paramsEntryRun.SetMinRowsVisible(2)
	envEntryRun := widget.NewMultiLineEntry()
	envEntryRun.SetPlaceHolder(`{"API_TOKEN":"xxxxx"}`)
	envEntryRun.SetMinRowsVisible(2)
	stdinEntryRun := widget.NewMultiLineEntry()
	stdinEntryRun.SetPlaceHolder("optional stdin...")
	stdinEntryRun.SetMinRowsVisible(2)
	testOut := widget.NewMultiLineEntry()
	testOut.Disable()
	testOut.SetMinRowsVisible(3)

	doTest := widget.NewButton("POST /run", func() {
		if runSelect.Selected == "" {
			dialog.ShowInformation("Test", "select a tool", w)
			return
		}
		go func() {
			var p map[string]any
			var e map[string]string
			if txt := strings.TrimSpace(paramsEntryRun.Text); txt != "" {
				if err := json.Unmarshal([]byte(txt), &p); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
			}
			if txt := strings.TrimSpace(envEntryRun.Text); txt != "" {
				if err := json.Unmarshal([]byte(txt), &e); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
			}
			body, _ := json.Marshal(model.RunRequest{Tool: runSelect.Selected, Params: p, Env: e, Stdin: stdinEntryRun.Text})
			urlStr := fmt.Sprintf("http://localhost%s%s%s", addrEntry.Text, baseEntry.Text, runEntry.Text)
			req, _ := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			if k := strings.TrimSpace(keyEntry.Text); k != "" {
				req.Header.Set("X-API-Key", k)
			}
			resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
			if err != nil {
				fyne.Do(func() { testOut.SetText("ERROR: " + err.Error()) })
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			fyne.Do(func() { testOut.SetText(fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(b))) })
		}()
	})

	runLabelWidth := widget.NewLabel("Params").MinSize().Width
	testPanel := container.NewBorder(widget.NewLabel("Test Run"), nil, nil, nil,
		withMargin(container.NewVBox(
			formRow("Tool", "", container.NewMax(runSelect), runLabelWidth),
			formRow("Params", "(JSON)", paramsEntryRun, runLabelWidth),
			formRow("Env", "(JSON)", envEntryRun, runLabelWidth),
			formRow("Stdin", "", container.NewBorder(nil, doTest, nil, nil, stdinEntryRun), runLabelWidth),
			formRow("Result", "", testOut, runLabelWidth),
		)),
	)

	logView := widget.NewMultiLineEntry()
	logView.Disable()
	logView.SetMinRowsVisible(5)
	logView.Wrapping = fyne.TextWrapWord
	go func() {
		t := time.NewTicker(300 * time.Millisecond)
		defer t.Stop()
		prev := ""
		for range t.C {
			txt := ring.String()
			if txt == prev {
				continue
			}
			prev = txt
			fyne.Do(func() { logView.SetText(txt) })
		}
	}()
	logPanel := container.NewBorder(widget.NewLabel("Server Logs"), nil, nil, nil, logView)

	startBtn := widget.NewButtonWithIcon("Start Server", theme.MediaPlayIcon(), func() {
		cfg := &model.ServerConfig{
			Addr:     strings.TrimSpace(addrEntry.Text),
			BasePath: strings.TrimSpace(baseEntry.Text),
			APIKey:   keyEntry.Text,
			CORS:     corsCheck.Checked,
			Tools:    tools,
			Paths:    model.Paths{Run: runEntry.Text, Tools: toolsEntry.Text, Reload: reloadEntry.Text, Health: healthEntry.Text},
		}
		if err := srv.Start(cfg); err != nil {
			dialog.ShowError(err, w)
			return
		}
		statusLbl.SetText("Server: running on " + cfg.Addr)
	})
	stopBtn := widget.NewButtonWithIcon("Stop Server", theme.MediaStopIcon(), func() {
		if err := srv.Stop(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		statusLbl.SetText("Server: stopped")
	})
	openHealth := widget.NewButton("Open Health", func() {
		u := fmt.Sprintf("http://localhost%s%s%s", addrEntry.Text, baseEntry.Text, healthEntry.Text)
		parsed, _ := url.Parse(u)
		_ = a.OpenURL(parsed)
	})

	leftForm := widget.NewForm(
		widget.NewFormItem("Addr", addrEntry),
		widget.NewFormItem("BasePath", baseEntry),
		widget.NewFormItem("Run", runEntry),
		widget.NewFormItem("Tools", toolsEntry),
		widget.NewFormItem("Reload", reloadEntry),
		widget.NewFormItem("Health", healthEntry),
		widget.NewFormItem("API Key", keyEntry),
		widget.NewFormItem("CORS", corsCheck),
	)

	// 入力は縦一列（左1/2）
	serverButtons := container.NewHBox(
		startBtn, stopBtn, openHealth,
	)
	serverPanel := container.NewVBox(
		widget.NewLabel("Server / API"),
		leftForm,
		serverButtons,
		statusLbl, // ステータスはボタンの下
	)

	// 右1/2に Test Run、下1/4 に Server Logs（幅変更バーなし）
	homeTop := container.NewGridWithColumns(2, serverPanel, testPanel)
	homeBody := container.NewBorder(nil, logPanel, nil, nil, homeTop)

	// 左カラムの基準幅（主ラベル想定）を計測
	regLabelWidth := widget.NewLabel("MaxStderr").MinSize().Width
	inputForm := container.NewVBox(
		formRow("Name", "", nameEntry, regLabelWidth),
		formRow("Group", "", groupEntry, regLabelWidth),
		formRow("Cmd", "", cmdEntry, regLabelWidth),
		formRow("Args", "(comma)", argsEntry, regLabelWidth),
		formRow("Workdir", "", workdirEntry, regLabelWidth),
		// Env の注釈は 1 行・小さいフォントで右並び
		formRow("Env", "(KEY=VAL\nper line)", envEntry, regLabelWidth),
		formRow("AllowEnv", "(comma)", allowEnvEntry, regLabelWidth),
		formRow("Timeout", "", timeoutEntry, regLabelWidth),
		formRow("MaxStdout", "", maxOutEntry, regLabelWidth),
		formRow("MaxStderr", "", maxErrEntry, regLabelWidth),
		formRow("Stdin", "", stdinCheck, regLabelWidth),
	)
	buttonsRow1 := container.NewCenter(container.NewHBox(
		widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), addTemplate),
		widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), saveTool),
		widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), delTool),
	))
	buttonsRow2 := container.NewCenter(container.NewHBox(
		widget.NewButtonWithIcon("Import YAML", theme.FolderOpenIcon(), importYAML),
		widget.NewButtonWithIcon("Export YAML", theme.DocumentSaveIcon(), exportYAML),
	))
	inputButtons := container.NewVBox(buttonsRow1, buttonsRow2)
	inputScroll := container.NewVScroll(withMargin(container.NewVBox(inputForm, widget.NewSeparator(), inputButtons)))
	accHolderCard := widget.NewCard("Tools (by Group)", "", accHolder)

	quickSelect = widget.NewSelect(toolNames(), nil)
	qParams := widget.NewMultiLineEntry()
	qParams.SetPlaceHolder(`{"msg":"hello"}`)
	qEnv := widget.NewMultiLineEntry()
	qEnv.SetPlaceHolder(`{"API_TOKEN":"xxxxx"}`)
	qStdin := widget.NewMultiLineEntry()
	qStdin.SetPlaceHolder("optional stdin...")
	qOut := widget.NewMultiLineEntry()
	qOut.Disable()
	qRun := widget.NewButton("Quick Run (POST /run)", func() {
		sel := quickSelect.Selected
		if sel == "" && nameEntry.Text != "" {
			sel = nameEntry.Text
		}
		if sel == "" {
			dialog.ShowInformation("Quick Run", "select a tool", w)
			return
		}
		go func() {
			var p map[string]any
			var e map[string]string
			if txt := strings.TrimSpace(qParams.Text); txt != "" {
				if err := json.Unmarshal([]byte(txt), &p); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
			}
			if txt := strings.TrimSpace(qEnv.Text); txt != "" {
				if err := json.Unmarshal([]byte(txt), &e); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
			}
			body, _ := json.Marshal(model.RunRequest{Tool: sel, Params: p, Env: e, Stdin: qStdin.Text})
			urlStr := fmt.Sprintf("http://localhost%s%s%s", addrEntry.Text, baseEntry.Text, runEntry.Text)
			req, _ := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			if k := strings.TrimSpace(keyEntry.Text); k != "" {
				req.Header.Set("X-API-Key", k)
			}
			resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
			if err != nil {
				fyne.Do(func() { qOut.SetText("ERROR: " + err.Error()) })
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			fyne.Do(func() { qOut.SetText(fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(b))) })
		}()
	})
	qFill := widget.NewButton("Fill Name/Cmd", func() {
		sel := quickSelect.Selected
		if sel == "" {
			dialog.ShowInformation("Quick CMD", "select a tool", w)
			return
		}
		if t, ok := tools[sel]; ok {
			nameEntry.SetText(sel)
			cmdEntry.SetText(t.Cmd)
		}
	})
	qTitle := container.NewBorder(nil, nil, widget.NewLabel("Quick CMD"), qFill, nil)

	quickPanel := container.NewBorder(
		qTitle, nil, nil, nil,
		withMargin(container.NewVBox(
			container.NewGridWithColumns(2, widget.NewLabel("Tool"), quickSelect),
			widget.NewLabel("Params (JSON)"), qParams,
			widget.NewLabel("Env (JSON)"), qEnv,
			widget.NewLabel("Stdin"), qStdin,
			qRun,
			widget.NewLabel("Result"), qOut,
		)),
	)

	// 入力を広く取り、ツール一覧を狭める
	registryLeft := container.NewBorder(nil, nil, nil, accHolderCard, inputScroll)
	registryBody := container.NewBorder(nil, nil, nil, quickPanel, registryLeft)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Server / API", theme.HomeIcon(), homeBody),
		container.NewTabItemWithIcon("Tools (Registry)", theme.SettingsIcon(), registryBody),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.Resize(fyne.NewSize(1024, 540))

	w.SetCloseIntercept(func() {
		a.Preferences().SetString("addr", addrEntry.Text)
		a.Preferences().SetString("base", baseEntry.Text)
		a.Preferences().SetString("path.run", runEntry.Text)
		a.Preferences().SetString("path.tools", toolsEntry.Text)
		a.Preferences().SetString("path.reload", reloadEntry.Text)
		a.Preferences().SetString("path.health", healthEntry.Text)
		a.Preferences().SetString("api_key", keyEntry.Text)
		a.Preferences().SetBool("cors", corsCheck.Checked)
		_ = srv.Stop()
		w.Close()
	})

	w.ShowAndRun()
}
