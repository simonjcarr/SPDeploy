//go:build windows

package daemon

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}