//go:build !windows

package shell

import (
	"fmt"
	"time"
)

type ServiceController interface {
	Stop(name string, timeout time.Duration) error
	Start(name string, timeout time.Duration) error
}

type UnsupportedServiceController struct{}

func NewServiceController() ServiceController {
	return UnsupportedServiceController{}
}

func (c UnsupportedServiceController) Stop(name string, timeout time.Duration) error {
	_ = name
	_ = timeout
	return fmt.Errorf("windows service control is not supported")
}

func (c UnsupportedServiceController) Start(name string, timeout time.Duration) error {
	_ = name
	_ = timeout
	return fmt.Errorf("windows service control is not supported")
}
