package main

// #cgo LDFLAGS: -shared
// #include <stdlib.h>
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/status-im/status-keycard-go/signal"
)

func main() {}

type api int

const (
	none api = iota
	flowAPI
	sessionAPI
)

func checkAPIMutualExclusion(requestedAPI api) error {
	switch requestedAPI {
	case flowAPI:
		if globalRPCServer != nil {
			return errors.New("not allowed to start flow API when session API is being used")
		}
	case sessionAPI:
		if globalFlow != nil {
			return errors.New("not allowed to start session API when flow API is being used")
		}
	default:
		panic("Unknown API")
	}

	return nil
}

//export KeycardSetSignalEventCallback
func KeycardSetSignalEventCallback(cb unsafe.Pointer) {
	signal.KeycardSetSignalEventCallback(cb)
}

//export ResetAPI
func ResetAPI() {
	globalFlow = nil
	globalRPCServer = nil
}

//export Free
func Free(param unsafe.Pointer) {
	C.free(param)
}

var initOnce sync.Once

//export InitializeStatusKeycardGo
func InitializeStatusKeycardGo() {
	initOnce.Do(func() {
		fmt.Println("👉 status-keycard-go: Starting Go runtime initialization")
		defer fmt.Println("✅ status-keycard-go: Go runtime initialization complete")

		// Force a garbage collection to initialize GC state.
		runtime.GC()

		// Spawn a dummy goroutine and wait for it to finish.
		done := make(chan struct{})
		go func() {
			// Minimal work just to kick off the scheduler.
			close(done)
		}()
		<-done
	})
}
