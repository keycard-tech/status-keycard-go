package main

// #cgo LDFLAGS: -shared
// #include <stdlib.h>
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/pkg/flow"
	"github.com/status-im/status-keycard-go/pkg/flow/mocked"
)

func main() {}

var globalFlow *mocked.MockedKeycardFlow

func retErr(err error) *C.char {
	if err == nil {
		return C.CString("ok")
	} else {
		return C.CString(err.Error())
	}
}

func jsonToParams(jsonParams *C.char) (flow.FlowParams, error) {
	var params flow.FlowParams

	if err := json.Unmarshal([]byte(C.GoString(jsonParams)), &params); err != nil {
		return nil, err
	}

	return params, nil
}

func jsonToMockedKeycard(jsonKeycard *C.char) (*mocked.MockedKeycard, error) {
	bytes := []byte(C.GoString(jsonKeycard))
	if len(bytes) == 0 {
		return nil, nil
	}

	mockedKeycard := &mocked.MockedKeycard{}
	if err := json.Unmarshal(bytes, mockedKeycard); err != nil {
		return nil, err
	}

	return mockedKeycard, nil
}

//export KeycardInitFlow
func KeycardInitFlow(storageDir *C.char) *C.char {
	var err error

	globalFlow, err = mocked.NewMockedFlow(C.GoString(storageDir))

	return retErr(err)
}

//export KeycardStartFlow
func KeycardStartFlow(flowType C.int, jsonParams *C.char) *C.char {
	params, err := jsonToParams(jsonParams)

	if err != nil {
		return retErr(err)
	}

	err = globalFlow.Start(flow.FlowType(flowType), params)
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
	mockedKeycardInst, err := jsonToMockedKeycard(mockedKeycard)
	if err != nil {
		return retErr(err)
	}

	mockedKeycardHelperInst, err := jsonToMockedKeycard(mockedKeycardHelper)
	if err != nil {
		return retErr(err)
	}

	err = globalFlow.RegisterKeycard(int(cardIndex), mocked.MockedReaderState(readerState), mocked.MockedKeycardState(keycardState),
		mockedKeycardInst, mockedKeycardHelperInst)
	return retErr(err)
}

//export MockedLibReaderPluggedIn
func MockedLibReaderPluggedIn() *C.char {
	err := globalFlow.ReaderPluggedIn()
	return retErr(err)
}

//export MockedLibReaderUnplugged
func MockedLibReaderUnplugged() *C.char {
	err := globalFlow.ReaderUnplugged()
	return retErr(err)
}

//export MockedLibKeycardInserted
func MockedLibKeycardInserted(cardIndex C.int) *C.char {
	err := globalFlow.KeycardInserted(int(cardIndex))
	return retErr(err)
}

//export MockedLibKeycardRemoved
func MockedLibKeycardRemoved() *C.char {
	err := globalFlow.KeycardRemoved()
	return retErr(err)
}
