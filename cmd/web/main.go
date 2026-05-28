//go:generate goversioninfo -64 -o resource.syso versioninfo.json

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ini-web-tool/internal/core"
	"ini-web-tool/internal/shell"
)

type targetConfig struct {
	Key         string
	DisplayName string
	IniPath     string
	ServiceName string
}

type app struct {
	targets   map[string]targetConfig
	backupDir string
	webDir    string
	services  shell.ServiceController
}

type apiResponse struct {
	StatusError bool          `json:"statusError"`
	Message     string        `json:"message"`
	Logs        []string      `json:"logs,omitempty"`
	State       *statePayload `json:"state,omitempty"`
}

type statePayload struct {
	Card stateTarget `json:"card"`
	IRS  stateIRS    `json:"irs"`
	NM43 stateTarget `json:"nm43"`
}

type stateTarget struct {
	Com   *int   `json:"com,omitempty"`
	Error string `json:"error,omitempty"`
}

type stateIRS struct {
	Device1Com *int   `json:"device1Com,omitempty"`
	Device2Com *int   `json:"device2Com,omitempty"`
	UseDevice2 bool   `json:"useDevice2"`
	Error      string `json:"error,omitempty"`
}

type inspectionStartTarget struct {
	Selected bool `json:"selected"`
}

type inspectionStartRequest struct {
	Card inspectionStartTarget `json:"card"`
	IRS  inspectionStartTarget `json:"irs"`
	NM43 inspectionStartTarget `json:"nm43"`
}

var inspectionStopServices = []string{
	"almfnclg",
	"almhlpcd",
	"almhlpld",
	"almhlppr",
	"almhlpss",
	"almhlpsd",
	"almhlptm",
	"texcashctl",
	"texct",
	"texdt",
	"texms",
	"texmy",
	"texpay",
	"texst",
	"almfncky",
	"texcs",
	"almfncad",
	"almfncpc",
	"almfncsc",
	"almdevpp1",
	"almdevcm1",
	"almdevcl9",
	"almdevca7",
	"almdevic2",
	"almdevmx1",
	"almdevic5",
	"almdevps1",
	"almdevqr6",
	"almdevsd1",
	"almdevhd1",
	"almdevcd7",
}

var inspectionStartServices = []string{
	"almdevcl9",
	"almdevca7",
	"almdevic2",
	"almdevmx1",
	"almdevps1",
	"almdevqr6",
	"almdevsd1",
	"almdevcm1",
	"almdevhd1",
	"almdevic5",
	"almdevcd7",
}

const inspectionToolPath = `d:\almex\tool\mark1_inspection\mark1_inspection.exe`
const appLogFileName = "ini-web-tool.log"
const nm43ServiceName = "almdevic5"
const inspectionLaunchDelayAfterNM43Start = 2 * time.Second

func main() {
	logPath, err := configureLogging(os.Getenv("ALMEXPATH"))
	if err != nil {
		log.Printf("ログ初期化失敗: %v", err)
	} else if logPath != "" {
		log.Printf("ログ出力先: %s", logPath)
	}

	application, err := newApp()
	if err != nil {
		log.Fatalf("起動失敗: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", application.handleState)
	mux.HandleFunc("/api/apply", application.handleApply)
	mux.HandleFunc("/api/inspection/start", application.handleInspectionStart)
	mux.Handle("/", http.FileServer(http.Dir(application.webDir)))

	address := "127.0.0.1:18080"
	url := "http://" + address
	log.Printf("Web UI: %s", url)
	openBrowser(url)
	if err := http.ListenAndServe(address, mux); err != nil {
		log.Fatalf("サーバー停止: %v", err)
	}
}

func newApp() (*app, error) {
	almexPath := os.Getenv("ALMEXPATH")
	if almexPath == "" {
		return nil, errors.New("ALMEXPATH が未定義です")
	}
	iniDir := filepath.Join(almexPath, "ini")
	if !isDir(iniDir) {
		return nil, fmt.Errorf("ini フォルダが見つかりません: %s", iniDir)
	}
	targets := map[string]targetConfig{
		core.TargetCard: {
			Key:         core.TargetCard,
			DisplayName: "almex_card_crl31.ini",
			IniPath:     filepath.Join(iniDir, "almex_card_crl31.ini"),
			ServiceName: "almdevcd7",
		},
		core.TargetIRS: {
			Key:         core.TargetIRS,
			DisplayName: "almex_iccard_irs270.ini",
			IniPath:     filepath.Join(iniDir, "almex_iccard_irs270.ini"),
			ServiceName: "almdevic2",
		},
		core.TargetNM43: {
			Key:         core.TargetNM43,
			DisplayName: "almex_iccard_nm43.ini",
			IniPath:     filepath.Join(iniDir, "almex_iccard_nm43.ini"),
			ServiceName: "almdevic5",
		},
	}
	for _, target := range targets {
		if !isFile(target.IniPath) {
			return nil, fmt.Errorf("ini ファイルが見つかりません: %s", target.IniPath)
		}
	}
	webDir, err := resolveWebDir()
	if err != nil {
		return nil, err
	}
	return &app{
		targets:   targets,
		backupDir: filepath.Join(iniDir, "backup"),
		webDir:    webDir,
		services:  shell.NewServiceController(),
	}, nil
}

func (a *app) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{
			StatusError: false,
			Message:     "GET のみ許可されています",
		})
		return
	}
	state, err := a.currentState()
	if err != nil {
		log.Printf("[state] 現在値取得失敗: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			StatusError: false,
			Message:     err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		StatusError: true,
		Message:     "現在値を取得しました",
		State:       state,
	})
}

func (a *app) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{
			StatusError: false,
			Message:     "POST のみ許可されています",
		})
		return
	}
	var request core.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("[apply] リクエスト形式不正: %v", err)
		writeJSON(w, http.StatusBadRequest, apiResponse{
			StatusError: false,
			Message:     "リクエスト形式が不正です",
		})
		return
	}
	operations, err := core.BuildOperations(request)
	if err != nil {
		log.Printf("[apply] 入力検証失敗: %v", err)
		writeJSON(w, http.StatusBadRequest, apiResponse{
			StatusError: false,
			Message:     err.Error(),
		})
		return
	}

	logs := make([]string, 0)
	for _, operation := range operations {
		lines, opErr := a.applyOperation(operation)
		logs = append(logs, lines...)
		if opErr != nil {
			logOperation("apply", opErr.Error(), logs)
			writeJSON(w, http.StatusInternalServerError, apiResponse{
				StatusError: false,
				Message:     opErr.Error(),
				Logs:        logs,
			})
			return
		}
	}

	state, err := a.currentState()
	if err != nil {
		logOperation("apply", err.Error(), logs)
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			StatusError: false,
			Message:     err.Error(),
			Logs:        logs,
		})
		return
	}
	logOperation("apply", "処理が完了しました", logs)
	writeJSON(w, http.StatusOK, apiResponse{
		StatusError: true,
		Message:     "処理が完了しました",
		Logs:        logs,
		State:       state,
	})
}

func (a *app) handleInspectionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{
			StatusError: false,
			Message:     "POST のみ許可されています",
		})
		return
	}

	request := defaultInspectionStartRequest()
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil &&
		!errors.Is(err, io.EOF) {
		log.Printf("[inspection/start] リクエスト形式不正: %v", err)
		writeJSON(w, http.StatusBadRequest, apiResponse{
			StatusError: false,
			Message:     "リクエスト形式が不正です",
		})
		return
	}
	log.Printf(
		"[inspection/start] request: card=%t irs=%t nm43=%t",
		request.Card.Selected,
		request.IRS.Selected,
		request.NM43.Selected,
	)

	logs := []string{"出荷検査事前処理を開始します"}
	serviceError := false
	startServices, skipLogs := buildInspectionStartServices(request)
	logs = append(logs, skipLogs...)
	results := shell.RunServiceActionsBestEffort(
		a.services,
		shell.BuildServiceActions(
			inspectionStopServices,
			startServices,
		),
		30*time.Second,
	)
	for _, result := range results {
		logLine, isError := inspectionServiceResultLog(result)
		logs = append(logs, logLine)
		if isError {
			serviceError = true
		}
	}

	if serviceError {
		logs = append(
			logs,
			"サービス失敗あり: 該当サービスをスキップして検査ツール起動を継続します",
		)
	}

	launchDelay := inspectionLaunchDelay(results)
	if launchDelay > 0 {
		logs = append(
			logs,
			fmt.Sprintf(
				"待機: %s 起動安定化のため %d秒",
				nm43ServiceName,
				int(launchDelay/time.Second),
			),
		)
		time.Sleep(launchDelay)
	}

	launchErr := launchInspectionTool()
	if launchErr != nil {
		logs = append(logs, fmt.Sprintf("起動失敗: %s", launchErr.Error()))
	} else {
		logs = append(
			logs,
			fmt.Sprintf("起動: %s -resetamount", inspectionToolPath),
		)
	}

	message := "出荷検査事前処理が完了しました"
	switch {
	case launchErr != nil && serviceError:
		message = "一部サービス操作に失敗し、検査ツール起動にも失敗しました。ログを確認してください"
	case launchErr != nil:
		message = "検査ツールの起動に失敗しました。ログを確認してください"
	case serviceError:
		message = "一部サービス操作に失敗しましたが、検査ツールを起動しました"
	}
	logOperation("inspection/start", message, logs)
	writeJSON(w, http.StatusOK, apiResponse{
		StatusError: launchErr == nil,
		Message:     message,
		Logs:        logs,
	})
}

func (a *app) currentState() (*statePayload, error) {
	card, err := a.loadBasicState(core.TargetCard)
	if err != nil {
		return nil, err
	}
	irs, err := a.loadIRSState()
	if err != nil {
		return nil, err
	}
	nm43, err := a.loadBasicState(core.TargetNM43)
	if err != nil {
		return nil, err
	}
	return &statePayload{
		Card: card,
		IRS:  irs,
		NM43: nm43,
	}, nil
}

func (a *app) loadBasicState(targetKey string) (stateTarget, error) {
	target := a.targets[targetKey]
	file, err := shell.ReadTextFile(target.IniPath)
	if err != nil {
		return stateTarget{}, err
	}
	value, parseErr := core.GetSectionComValue(file.Text, "DEVICE1")
	if parseErr != nil {
		return stateTarget{Error: parseErr.Error()}, nil
	}
	return stateTarget{Com: value}, nil
}

func (a *app) loadIRSState() (stateIRS, error) {
	target := a.targets[core.TargetIRS]
	file, err := shell.ReadTextFile(target.IniPath)
	if err != nil {
		return stateIRS{}, err
	}
	device1Com, err1 := core.GetSectionComValue(file.Text, "DEVICE1")
	device2Com, err2 := core.GetSectionComValue(file.Text, "DEVICE2")
	useDevice2 := !core.IsSectionAutoOff(file.Text, "DEVICE2")
	errorMessage := ""
	if err1 != nil {
		errorMessage = err1.Error()
	}
	if err1 == nil && err2 != nil {
		errorMessage = err2.Error()
	}
	return stateIRS{
		Device1Com: device1Com,
		Device2Com: device2Com,
		UseDevice2: useDevice2,
		Error:      errorMessage,
	}, nil
}

func (a *app) applyOperation(operation core.Operation) ([]string, error) {
	target, ok := a.targets[operation.Target]
	if !ok {
		return []string{}, fmt.Errorf("未対応対象: %s", operation.Target)
	}
	lines := []string{fmt.Sprintf("処理開始: %s", target.DisplayName)}
	source, err := shell.ReadTextFile(target.IniPath)
	if err != nil {
		return lines, err
	}

	updated, err := a.buildUpdatedText(operation, source.Text)
	if err != nil {
		return lines, err
	}
	if !updated.Changed {
		return append(lines, "変更なし"), nil
	}

	backupPath, err := shell.EnsureBackup(target.IniPath, a.backupDir)
	if err != nil {
		return lines, err
	}
	lines = append(lines, fmt.Sprintf("バックアップ作成: %s", backupPath))

	stopped := false
	saved := false
	if err := a.services.Stop(target.ServiceName, 30*time.Second); err != nil {
		lines = append(lines,
			fmt.Sprintf("サービス停止失敗: %s", err.Error()))
		return lines, wrapPermissionError(
			err,
			"サービス停止",
			target.ServiceName,
		)
	}
	stopped = true
	lines = append(lines, fmt.Sprintf("サービス停止: %s", target.ServiceName))

	if err := shell.WriteTextFile(target.IniPath, shell.TextFile{
		Text:     updated.Text,
		Encoding: source.Encoding,
	}); err != nil {
		lines = append(lines,
			fmt.Sprintf("ini 保存失敗: %s", err.Error()))
		_ = a.services.Start(target.ServiceName, 30*time.Second)
		return lines, wrapPermissionError(
			err,
			"ini 保存",
			target.DisplayName,
		)
	}
	saved = true
	lines = append(lines, "ini 保存完了")

	if err := a.services.Start(target.ServiceName, 30*time.Second); err != nil {
		lines = append(lines,
			fmt.Sprintf("サービス開始失敗: %s", err.Error()))
		_ = a.rollback(target, backupPath, stopped, saved)
		return lines, wrapPermissionError(
			err,
			"サービス開始",
			target.ServiceName,
		)
	}
	lines = append(lines, fmt.Sprintf("サービス開始: %s", target.ServiceName))
	return append(lines, "完了"), nil
}

func (a *app) rollback(
	target targetConfig,
	backupPath string,
	stopped bool,
	saved bool,
) error {
	if saved {
		if err := shell.RestoreBackup(backupPath, target.IniPath); err != nil {
			return err
		}
	}
	if stopped {
		if err := a.services.Start(target.ServiceName, 30*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func defaultInspectionStartRequest() inspectionStartRequest {
	return inspectionStartRequest{
		Card: inspectionStartTarget{Selected: true},
		IRS:  inspectionStartTarget{Selected: true},
		NM43: inspectionStartTarget{Selected: true},
	}
}

func buildInspectionStartServices(
	request inspectionStartRequest,
) ([]string, []string) {
	starts := make([]string, 0, len(inspectionStartServices))
	logs := make([]string, 0, 3)
	for _, service := range inspectionStartServices {
		if shouldSkipInspectionStartService(service, request) {
			logs = append(logs, inspectionStartSkipLog(service))
			continue
		}
		starts = append(starts, service)
	}
	return starts, logs
}

func inspectionLaunchDelay(results []shell.ServiceActionResult) time.Duration {
	if hasSuccessfulInspectionServiceStart(results, nm43ServiceName) {
		return inspectionLaunchDelayAfterNM43Start
	}
	return 0
}

func hasSuccessfulInspectionServiceStart(
	results []shell.ServiceActionResult,
	serviceName string,
) bool {
	for _, result := range results {
		if result.Action.Type != shell.ServiceActionStart {
			continue
		}
		if result.Action.Name != serviceName {
			continue
		}
		return result.Err == nil
	}
	return false
}

func shouldSkipInspectionStartService(
	service string,
	request inspectionStartRequest,
) bool {
	switch service {
	case "almdevcd7":
		return !request.Card.Selected
	case "almdevic2":
		return !request.IRS.Selected
	case "almdevic5":
		return !request.NM43.Selected
	default:
		return false
	}
}

func inspectionStartSkipLog(service string) string {
	switch service {
	case "almdevcd7":
		return "開始スキップ: almdevcd7 (Card 未選択)"
	case "almdevic2":
		return "開始スキップ: almdevic2 (IRS 未選択)"
	case "almdevic5":
		return "開始スキップ: almdevic5 (NM43 未選択)"
	default:
		return fmt.Sprintf("開始スキップ: %s", service)
	}
}

func (a *app) buildUpdatedText(operation core.Operation, text string) (
	core.PatchResult, error,
) {
	if operation.Target == core.TargetIRS {
		return core.UpdateIrsText(
			text,
			operation.Device1Com,
			operation.UseDevice2,
			operation.Device2Com,
		)
	}
	return core.SetSectionComValue(text, "DEVICE1", operation.Device1Com)
}

func writeJSON(w http.ResponseWriter, status int, payload apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func configureLogging(almexPath string) (string, error) {
	trimmed := strings.TrimSpace(almexPath)
	if trimmed == "" {
		return "", nil
	}
	logDir := filepath.Join(trimmed, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	logPath := filepath.Join(logDir, appLogFileName)
	file, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return "", err
	}
	log.SetOutput(io.MultiWriter(os.Stdout, file))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return logPath, nil
}

func logOperation(name string, message string, lines []string) {
	log.Printf("[%s] %s", name, message)
	for _, line := range lines {
		log.Printf("[%s] %s", name, line)
	}
}

func resolveWebDir() (string, error) {
	cwdWeb := filepath.Join(".", "web")
	if isDir(cwdWeb) {
		return cwdWeb, nil
	}
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeWeb := filepath.Join(filepath.Dir(exePath), "web")
	if isDir(exeWeb) {
		return exeWeb, nil
	}
	return "", errors.New("web フォルダが見つかりません")
}

func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	_ = cmd.Start()
}

func launchInspectionTool() error {
	cmd := exec.Command(inspectionToolPath, "-resetamount")
	return cmd.Start()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func wrapPermissionError(
	err error,
	action string,
	target string,
) error {
	message := err.Error()
	lower := strings.ToLower(message)
	if strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "openservice failed 5") ||
		strings.Contains(lower, "startservice failed 5") ||
		strings.Contains(lower, "stopservice failed 5") {
		return fmt.Errorf(
			"%sに失敗しました (%s): %s。"+
				"サービス操作権限を確認してください"+
				"（標準ユーザー運用時は事前権限付与が必要です）",
			action,
			target,
			message,
		)
	}
	return err
}

func inspectionServiceResultLog(
	result shell.ServiceActionResult,
) (string, bool) {
	action := "開始"
	prefix := "サービス開始"
	if result.Action.Type == shell.ServiceActionStop {
		action = "停止"
		prefix = "サービス停止"
	}
	if result.Err != nil {
		wrapped := wrapPermissionError(
			result.Err,
			prefix,
			result.Action.Name,
		)
		return fmt.Sprintf("%s失敗: %s", action, wrapped.Error()), true
	}
	return fmt.Sprintf("%s: %s", action, result.Action.Name), false
}
