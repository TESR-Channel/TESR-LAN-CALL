//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// ซ่อนหน้าต่าง console ของคำสั่งที่เรียก (reg, rundll32)
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
