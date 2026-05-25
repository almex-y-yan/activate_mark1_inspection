package shell

import (
	"errors"
	"testing"
	"time"
)

func TestBuildServiceActions_OrderAndType(t *testing.T) {
	stops := []string{"stop-a", "stop-b"}
	starts := []string{"start-a"}

	actions := BuildServiceActions(stops, starts)
	if len(actions) != 3 {
		t.Fatalf("actions len mismatch: got=%d want=3", len(actions))
	}

	assertAction(t, actions[0], "stop-a", ServiceActionStop)
	assertAction(t, actions[1], "stop-b", ServiceActionStop)
	assertAction(t, actions[2], "start-a", ServiceActionStart)
}

func TestBuildServiceActions_Empty(t *testing.T) {
	actions := BuildServiceActions([]string{}, []string{})
	if len(actions) != 0 {
		t.Fatalf("actions len mismatch: got=%d want=0", len(actions))
	}
}

func TestShouldAbortServiceActions_WhenCredentialUnavailable(t *testing.T) {
	err := errors.New("prefix: " + errServiceCredentialUnavailable.Error())
	if !shouldAbortServiceActions(errServiceCredentialUnavailable) {
		t.Fatalf("expected abort for credential unavailable")
	}
	if shouldAbortServiceActions(nil) {
		t.Fatalf("did not expect abort for nil error")
	}
	if shouldAbortServiceActions(err) {
		t.Fatalf("did not expect abort for non-wrapped error")
	}
}

func TestRunServiceActionsBestEffort_ContinuesAfterErrors(t *testing.T) {
	controller := &fakeServiceController{
		stopErrs: map[string]error{
			"stop-a": errors.New("OpenService FAILED 5: Access is denied."),
		},
	}

	results := RunServiceActionsBestEffort(
		controller,
		[]ServiceAction{
			{Name: "stop-a", Type: ServiceActionStop},
			{Name: "start-a", Type: ServiceActionStart},
			{Name: "start-b", Type: ServiceActionStart},
		},
		time.Second,
	)

	if len(results) != 3 {
		t.Fatalf("results len mismatch: got=%d want=3", len(results))
	}
	if len(controller.calls) != 3 {
		t.Fatalf("call len mismatch: got=%d want=3", len(controller.calls))
	}
}

type fakeServiceController struct {
	stopErrs  map[string]error
	startErrs map[string]error
	calls     []string
}

func (f *fakeServiceController) Stop(name string, _ time.Duration) error {
	f.calls = append(f.calls, "stop:"+name)
	if f.stopErrs == nil {
		return nil
	}
	return f.stopErrs[name]
}

func (f *fakeServiceController) Start(name string, _ time.Duration) error {
	f.calls = append(f.calls, "start:"+name)
	if f.startErrs == nil {
		return nil
	}
	return f.startErrs[name]
}

func assertAction(
	t *testing.T,
	got ServiceAction,
	wantName string,
	wantType ServiceActionType,
) {
	t.Helper()
	if got.Name != wantName {
		t.Fatalf("action name mismatch: got=%s want=%s", got.Name, wantName)
	}
	if got.Type != wantType {
		t.Fatalf("action type mismatch: got=%s want=%s", got.Type, wantType)
	}
}
