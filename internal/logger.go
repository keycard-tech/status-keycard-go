package internal

import (
	"fmt"
)

// Deprecated: Printf is deprecated, use zap logger instead
func Printf(format string, args ...interface{}) {
	f := fmt.Sprintf("keycard - %s\n", format)
	fmt.Printf(f, args...)
}
