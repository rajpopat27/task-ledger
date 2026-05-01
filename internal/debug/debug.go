package debug

import (
	"fmt"
	"os"
)

var (
	enabled     = os.Getenv("TL_DEBUG") != ""
	verboseMode = false
	quietMode   = false
)

// SetVerbose enables verbose/debug output
func SetVerbose(verbose bool) {
	verboseMode = verbose
}

// SetQuiet enables quiet mode (suppress non-essential output)
func SetQuiet(quiet bool) {
	quietMode = quiet
}

// IsQuiet returns true if quiet mode is enabled
func IsQuiet() bool {
	return quietMode
}

func Logf(format string, args ...interface{}) {
	if enabled || verboseMode {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}
