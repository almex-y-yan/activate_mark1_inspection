//go:build windows

package shell

import "errors"

var errServiceCredentialUnavailable = errors.New(
	"service credential unavailable",
)

func isServiceCredentialUnavailable(err error) bool {
	return errors.Is(err, errServiceCredentialUnavailable)
}

func isServiceAccessDenied(err error) bool {
	return isAccessDenied(err)
}
