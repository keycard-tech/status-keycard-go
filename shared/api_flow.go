package main

// #cgo LDFLAGS: -shared
// #include <stdlib.h>
import "C"

import (
	"encoding/json"
	"errors"

	"github.com/status-im/status-keycard-go/pkg/flow"
)

var (
	notAvailable   = errors.New("not available in this context")
	notInitialized = errors.New("flow not initialized")
)

var globalFlow *flow.KeycardFlow

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

//export KeycardInitFlow
func KeycardInitFlow(storageDir *C.char) *C.char {
	if err := checkAPIMutualExclusion(flowAPI); err != nil {
		return retErr(err)
	}

	var err error
	globalFlow, err = flow.NewFlow(C.GoString(storageDir))

	return retErr(err)
}

//export KeycardStartFlow
func KeycardStartFlow(flowType C.int, jsonParams *C.char) *C.char {
	if globalFlow == nil {
		return retErr(notInitialized)
	}

	params, err := jsonToParams(jsonParams)

	if err != nil {
		return retErr(err)
	}

	err = globalFlow.Start(flow.FlowType(flowType), params)
	return retErr(err)
}

//export KeycardResumeFlow
func KeycardResumeFlow(jsonParams *C.char) *C.char {
	if globalFlow == nil {
		return retErr(notInitialized)
	}

	params, err := jsonToParams(jsonParams)

	if err != nil {
		return retErr(err)
	}

	err = globalFlow.Resume(params)
	return retErr(err)
}

//export KeycardCancelFlow
func KeycardCancelFlow() *C.char {
	if globalFlow == nil {
		return retErr(notInitialized)
	}

	err := globalFlow.Cancel()
	return retErr(err)
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
