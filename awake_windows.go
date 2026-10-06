//go:build windows

package main

import "syscall"

var setThreadExecState = syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// ไม่ให้คอมเข้าโหมด Sleep ระหว่างเปิดโปรแกรม (จอดับได้ตามปกติ) เรียกซ้ำทุก 30 วินาทีเพื่อรีเซ็ตตัวนับเวลาว่าง
func preventSleep(on bool) {
	if on {
		_, _, _ = setThreadExecState.Call(uintptr(0x00000001)) // ES_SYSTEM_REQUIRED
	}
}
