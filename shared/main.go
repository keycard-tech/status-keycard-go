package main

// #cgo LDFLAGS: -shared
// #include <stdlib.h>
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	skg "github.com/status-im/status-keycard-go"
	"github.com/status-im/status-keycard-go/signal"
)

func main() {}

var notAvailable = errors.New("not available in this context")
var ok = errors.New("ok")

var globalFlow *skg.KeycardFlow

func retErr(err error) *C.char {
	if err == nil {
		return C.CString(ok.Error())
	} else {
		return C.CString(err.Error())
	}
}

func jsonToParams(jsonParams *C.char) (skg.FlowParams, error) {
	var params skg.FlowParams

	if err := json.Unmarshal([]byte(C.GoString(jsonParams)), &params); err != nil {
		return nil, err
	}

	return params, nil
}

//export KeycardInitFlow
func KeycardInitFlow(storageDir *C.char) *C.char {
	var err error
	l("before skg.NewFlow(C.GoString(storageDir))")
	l("value of storageDir is -> %v", storageDir)
	globalFlow, err = skg.NewFlow(C.GoString(storageDir))
	l("error is %+v", err)

	return retErr(err)
}

//export KeycardStartFlow
func KeycardStartFlow(flowType C.int, jsonParams *C.char) *C.char {
	params, err := jsonToParams(jsonParams)

	if err != nil {
		return retErr(err)
	}

	err = globalFlow.Start(skg.FlowType(flowType), params)
	return retErr(err)
}

//export KeycardResumeFlow
func KeycardResumeFlow(jsonParams *C.char) *C.char {
	params, err := jsonToParams(jsonParams)

	if err != nil {
		return retErr(err)
	}

	err = globalFlow.Resume(params)
	return retErr(err)
}

//export KeycardCancelFlow
func KeycardCancelFlow() *C.char {
	err := globalFlow.Cancel()
	return retErr(err)
}

//export Free
func Free(param unsafe.Pointer) {
	C.free(param)
}

//export KeycardSetSignalEventCallback
func KeycardSetSignalEventCallback(cb unsafe.Pointer) {
	signal.KeycardSetSignalEventCallback(cb)
}

//export MockedLibRegisterKeycard
func MockedLibRegisterKeycard(cardIndex C.int, readerState C.int, keycardState C.int, mockedKeycard *C.char, mockedKeycardHelper *C.char) *C.char {
	return retErr(notAvailable)
}

//export MockedLibReaderPluggedIn
func MockedLibReaderPluggedIn() *C.char {
	return retErr(notAvailable)
}

//export MockedLibReaderUnplugged
func MockedLibReaderUnplugged() *C.char {
	return retErr(notAvailable)
}

//export MockedLibKeycardInserted
func MockedLibKeycardInserted(cardIndex C.int) *C.char {
	return retErr(notAvailable)
}

//export MockedLibKeycardRemoved
func MockedLibKeycardRemoved() *C.char {
	return retErr(notAvailable)
}

func l(format string, args ...interface{}) {
	f := fmt.Sprintf("keycard - %s\n", format)
	fmt.Printf(f, args...)
}
