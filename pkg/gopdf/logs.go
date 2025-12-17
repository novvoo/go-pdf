package gopdf

import (
	"fmt"
	"io"
	"os"
)

// debugOutput 调试输出目标
var debugOutput io.Writer = os.Stdout

// debugEnabled 是否启用调试输出
var debugEnabled = false

// SetDebugOutput 设置调试输出目标
func SetDebugOutput(w io.Writer) {
	debugOutput = w
	debugEnabled = true
}

// EnableDebug 启用调试输出
func EnableDebug() {
	debugEnabled = true
}

// DisableDebug 禁用调试输出
func DisableDebug() {
	debugEnabled = false
}

// debugPrintf 调试输出函数
func debugPrintf(format string, args ...interface{}) {
	if debugEnabled && debugOutput != nil {
		fmt.Fprintf(debugOutput, format, args...)
	}
}

// debugPrintln 调试输出函数（带换行）
func debugPrintln(args ...interface{}) {
	if debugEnabled && debugOutput != nil {
		fmt.Fprintln(debugOutput, args...)
	}
}
