//go:build windows

package shell

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type ServiceController interface {
	Stop(name string, timeout time.Duration) error
	Start(name string, timeout time.Duration) error
}

type WindowsServiceController struct{}

const (
	servicePollInterval = 500 * time.Millisecond
	elevationUser       = "almexad-02"
	elevationPassword   = "Alme@Xme009748#"
	envElevationUser    = "INI_TOOL_ELEVATION_USER"
	envElevationPass    = "INI_TOOL_ELEVATION_PASSWORD"
	logonInteractive    = 2
	logonProvider       = 0
)

var (
	modAdvapi32                 = windows.NewLazySystemDLL("advapi32.dll")
	procLogonUserW              = modAdvapi32.NewProc("LogonUserW")
	procImpersonateLoggedOnUser = modAdvapi32.NewProc(
		"ImpersonateLoggedOnUser",
	)
	procDuplicateTokenEx = modAdvapi32.NewProc("DuplicateTokenEx")
	procRevertToSelf = modAdvapi32.NewProc("RevertToSelf")
)

func NewServiceController() ServiceController {
	return WindowsServiceController{}
}

func (c WindowsServiceController) Stop(name string, timeout time.Duration) error {
	return runWindowsServiceAction("stop", name, timeout)
}

func (c WindowsServiceController) Start(name string, timeout time.Duration) error {
	return runWindowsServiceAction("start", name, timeout)
}

func runWindowsServiceAction(
	action string,
	serviceName string,
	timeout time.Duration,
) error {
	if strings.TrimSpace(serviceName) == "" {
		return fmt.Errorf("サービス名が空です")
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	expectedState, err := expectedStateForAction(action)
	if err != nil {
		return err
	}
	currentState, currentErr := queryServiceState(serviceName)
	if currentErr == nil &&
		strings.EqualFold(strings.TrimSpace(currentState), expectedState) {
		return nil
	}
	if err := invokeServiceAction(action, serviceName); err != nil {
		return err
	}
	return waitForServiceState(serviceName, expectedState, timeout)
}

func expectedStateForAction(action string) (string, error) {
	if strings.EqualFold(action, "stop") {
		return "STOPPED", nil
	}
	if strings.EqualFold(action, "start") {
		return "RUNNING", nil
	}
	return "", fmt.Errorf("未対応アクション: %s", action)
}

func invokeServiceAction(action string, serviceName string) error {
	return withService(serviceName, func(service *mgr.Service) error {
		if strings.EqualFold(action, "stop") {
			return stopService(service)
		}
		return startService(service)
	})
}

func waitForServiceState(
	serviceName string,
	expectedState string,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	lastState := ""
	lastErr := error(nil)
	for {
		state, err := queryServiceState(serviceName)
		if err == nil {
			lastState = state
			if strings.EqualFold(strings.TrimSpace(state), expectedState) {
				return nil
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("状態確認失敗: %w", lastErr)
			}
			return fmt.Errorf(
				"状態遷移失敗: expected=%s actual=%s",
				expectedState,
				lastState,
			)
		}
		time.Sleep(servicePollInterval)
	}
}

func queryServiceState(serviceName string) (string, error) {
	state, err := queryServiceStateRaw(serviceName)
	if err != nil {
		return "", err
	}
	stateText, ok := serviceStateText[state]
	if !ok {
		return "", fmt.Errorf("unknown service state: %d", state)
	}
	return stateText, nil
}

var serviceStateText = map[svc.State]string{
	svc.Stopped:         "STOPPED",
	svc.StartPending:    "START_PENDING",
	svc.StopPending:     "STOP_PENDING",
	svc.Running:         "RUNNING",
	svc.ContinuePending: "CONTINUE_PENDING",
	svc.PausePending:    "PAUSE_PENDING",
	svc.Paused:          "PAUSED",
}

func queryServiceStateRaw(serviceName string) (svc.State, error) {
	currentState := svc.Stopped
	err := withService(serviceName, func(service *mgr.Service) error {
		status, queryErr := service.Query()
		if queryErr != nil {
			return queryErr
		}
		currentState = status.State
		return nil
	})
	if err != nil {
		return svc.Stopped, err
	}
	return currentState, nil
}

func withService(
	serviceName string,
	handle func(*mgr.Service) error,
) error {
	trimmedName := strings.TrimSpace(serviceName)
	if trimmedName == "" {
		return errors.New("サービス名が空です")
	}
	operation := func() error {
		manager, err := mgr.Connect()
		if err != nil {
			return err
		}
		defer manager.Disconnect()
		service, err := manager.OpenService(trimmedName)
		if err != nil {
			return err
		}
		defer service.Close()
		return handle(service)
	}
	err := operation()
	if err == nil {
		return nil
	}
	if !isAccessDenied(err) {
		return err
	}
	return runWithElevatedImpersonation(operation)
}

func runWithElevatedImpersonation(run func() error) error {
	candidates := credentialCandidates()
	lastErr := error(nil)
	for _, c := range candidates {
		primaryToken, err := loginAs(c.user, c.password)
		if err != nil {
			lastErr = err
			if errors.Is(err, windows.ERROR_ACCOUNT_DISABLED) {
				return fmt.Errorf(
					"%w: 管理者ログオンに失敗: %s",
					errServiceCredentialUnavailable,
					err.Error(),
				)
			}
			continue
		}
		impersonationToken, err := duplicateAsImpersonationToken(primaryToken)
		primaryToken.Close()
		if err != nil {
			lastErr = err
			continue
		}
		defer impersonationToken.Close()
		if err := impersonateLoggedOnUser(impersonationToken); err != nil {
			lastErr = err
			continue
		}
		defer revertToSelf()
		return run()
	}
	if lastErr != nil {
		return fmt.Errorf(
			"%w: 管理者権限の取得に失敗: %v",
			errServiceCredentialUnavailable,
			lastErr,
		)
	}
	return fmt.Errorf(
		"%w: 管理者ログオンに失敗: 資格情報が未設定です",
		errServiceCredentialUnavailable,
	)
}

func startService(service *mgr.Service) error {
	err := service.Start()
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return nil
	}
	return err
}

func stopService(service *mgr.Service) error {
	_, err := service.Control(svc.Stop)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return nil
	}
	return err
}

func isAccessDenied(err error) bool {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "access is denied")
}

type serviceCredential struct {
	user     string
	password string
}

func credentialCandidates() []serviceCredential {
	candidates := make([]serviceCredential, 0, 2)
	envUser := strings.TrimSpace(os.Getenv(envElevationUser))
	envPass := os.Getenv(envElevationPass)
	if envUser != "" && envPass != "" {
		candidates = append(candidates, serviceCredential{
			user:     envUser,
			password: envPass,
		})
	}
	candidates = append(candidates, serviceCredential{
		user:     elevationUser,
		password: elevationPassword,
	})
	return candidates
}

func loginAs(userName string, rawPassword string) (windows.Token, error) {
	user, err := windows.UTF16PtrFromString(userName)
	if err != nil {
		return 0, err
	}
	password, err := windows.UTF16PtrFromString(rawPassword)
	if err != nil {
		return 0, err
	}
	domain, err := windows.UTF16PtrFromString(".")
	if err != nil {
		return 0, err
	}
	return logonUser(
		user,
		domain,
		password,
		logonInteractive,
		logonProvider,
	)
}

func logonUser(
	user *uint16,
	domain *uint16,
	password *uint16,
	logonType uint32,
	logonProviderValue uint32,
) (windows.Token, error) {
	var token windows.Token
	r1, _, callErr := procLogonUserW.Call(
		uintptr(unsafe.Pointer(user)),
		uintptr(unsafe.Pointer(domain)),
		uintptr(unsafe.Pointer(password)),
		uintptr(logonType),
		uintptr(logonProviderValue),
		uintptr(unsafe.Pointer(&token)),
	)
	if r1 != 0 {
		return token, nil
	}
	if callErr == syscall.Errno(0) {
		return 0, syscall.EINVAL
	}
	return 0, callErr
}

func impersonateLoggedOnUser(token windows.Token) error {
	r1, _, callErr := procImpersonateLoggedOnUser.Call(uintptr(token))
	if r1 != 0 {
		return nil
	}
	if callErr == syscall.Errno(0) {
		return syscall.EINVAL
	}
	return callErr
}

func duplicateAsImpersonationToken(
	primary windows.Token,
) (windows.Token, error) {
	var duplicated windows.Token
	const (
		securityImpersonation = 2
		tokenTypeImpersonation = 2
	)
	r1, _, callErr := procDuplicateTokenEx.Call(
		uintptr(primary),
		uintptr(windows.MAXIMUM_ALLOWED),
		0,
		uintptr(securityImpersonation),
		uintptr(tokenTypeImpersonation),
		uintptr(unsafe.Pointer(&duplicated)),
	)
	if r1 != 0 {
		return duplicated, nil
	}
	if callErr == syscall.Errno(0) {
		return 0, syscall.EINVAL
	}
	return 0, callErr
}

func revertToSelf() {
	_, _, _ = procRevertToSelf.Call()
}
