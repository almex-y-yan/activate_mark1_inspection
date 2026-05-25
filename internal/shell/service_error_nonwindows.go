//go:build !windows

package shell

func isServiceCredentialUnavailable(err error) bool {
	_ = err
	return false
}

func isServiceAccessDenied(err error) bool {
	_ = err
	return false
}
