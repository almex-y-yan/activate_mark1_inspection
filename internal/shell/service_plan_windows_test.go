//go:build windows

package shell

import (
	"errors"
	"testing"
)

func TestShouldAbortServiceActions_WhenAccessDenied(t *testing.T) {
	denied := errors.New("OpenService FAILED 5: Access is denied.")
	if !shouldAbortServiceActions(denied) {
		t.Fatalf("expected abort for access denied")
	}
}
