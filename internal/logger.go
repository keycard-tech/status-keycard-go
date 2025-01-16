package internal

import "fmt"

func Printf(format string, args ...interface{}) {
	f := fmt.Sprintf("keycard - %s\n", format)
	fmt.Printf(f, args...)
}
