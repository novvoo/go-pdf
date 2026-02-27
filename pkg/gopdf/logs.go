package gopdf

import (
	"fmt"
	"io"
	"os"
)

func SetDebugOutput(w io.Writer) {
	logger := GetLogger()
	logger.SetOutput(w)
	logger.SetLevel(LogLevelDebug)
	logger.SetEnabled(true)
}

func EnableDebug() {
	logger := GetLogger()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(LogLevelDebug)
	logger.SetEnabled(true)
}

func DisableDebug() {
	logger := GetLogger()
	logger.SetLevel(LogLevelWarn)
}

func debugPrintf(format string, args ...interface{}) {
	Debug(format, args...)
}

func debugPrintln(args ...interface{}) {
	Debug("%s", fmt.Sprintln(args...))
}

