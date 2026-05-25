package shell

import "time"

type ServiceActionType string

const (
	ServiceActionStop  ServiceActionType = "stop"
	ServiceActionStart ServiceActionType = "start"
)

type ServiceAction struct {
	Name string
	Type ServiceActionType
}

type ServiceActionResult struct {
	Action ServiceAction
	Err    error
}

func BuildServiceActions(stops []string, starts []string) []ServiceAction {
	actions := make([]ServiceAction, 0, len(stops)+len(starts))
	for _, name := range stops {
		actions = append(actions, ServiceAction{
			Name: name,
			Type: ServiceActionStop,
		})
	}
	for _, name := range starts {
		actions = append(actions, ServiceAction{
			Name: name,
			Type: ServiceActionStart,
		})
	}
	return actions
}

func RunServiceActions(
	controller ServiceController,
	actions []ServiceAction,
	timeout time.Duration,
) []ServiceActionResult {
	results := make([]ServiceActionResult, 0, len(actions))
	for _, action := range actions {
		err := runServiceAction(controller, action, timeout)
		results = append(results, ServiceActionResult{
			Action: action,
			Err:    err,
		})
		if shouldAbortServiceActions(err) {
			break
		}
	}
	return results
}

func RunServiceActionsBestEffort(
	controller ServiceController,
	actions []ServiceAction,
	timeout time.Duration,
) []ServiceActionResult {
	results := make([]ServiceActionResult, 0, len(actions))
	for _, action := range actions {
		err := runServiceAction(controller, action, timeout)
		results = append(results, ServiceActionResult{
			Action: action,
			Err:    err,
		})
	}
	return results
}

func runServiceAction(
	controller ServiceController,
	action ServiceAction,
	timeout time.Duration,
) error {
	if action.Type == ServiceActionStop {
		return controller.Stop(action.Name, timeout)
	}
	return controller.Start(action.Name, timeout)
}

func shouldAbortServiceActions(err error) bool {
	if err == nil {
		return false
	}
	if isServiceCredentialUnavailable(err) {
		return true
	}
	return isServiceAccessDenied(err)
}
