package main

import "C"
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"

	"github.com/gorilla/rpc"
	"github.com/pkg/errors"

	"github.com/status-im/status-keycard-go/pkg/session"
)

var (
	globalRPCServer *rpc.Server
)

func marshalError(err error) *C.char {
	response := struct {
		Error string `json:"error"`
	}{
		Error: "",
	}
	if err != nil {
		response.Error = err.Error()
	}
	responseBytes, _ := json.Marshal(response)
	return C.CString(string(responseBytes))
}

//export KeycardInitializeRPC
func KeycardInitializeRPC() *C.char {
	if err := checkAPIMutualExclusion(sessionAPI); err != nil {
		return marshalError(err)
	}

	rpcServer, err := session.CreateRPCServer()
	if err != nil {
		return marshalError(err)
	}
	globalRPCServer = rpcServer
	return marshalError(nil)
}

//export KeycardCallRPC
func KeycardCallRPC(payload *C.char) *C.char {
	if globalRPCServer == nil {
		return marshalError(errors.New("RPC server not initialized"))
	}

	payloadBytes := []byte(C.GoString(payload))

	// Create a fake HTTP request
	req := httptest.NewRequest("POST", "/rpc", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	// Create a fake HTTP response writer
	rr := httptest.NewRecorder()

	// Call the server's ServeHTTP method
	globalRPCServer.ServeHTTP(rr, req)

	// Read and return the response body
	resp := rr.Result()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return marshalError(errors.Wrap(err, "internal error reading response body"))
	}

	return C.CString(string(body))
}
