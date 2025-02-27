package main

import "C"
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"

	"github.com/gorilla/rpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"

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

func logPanic() {
	err := recover()
	if err != nil {
		fmt.Printf("Panic: %v\n", err)
	}
}

//export KeycardInitializeRPC
func KeycardInitializeRPC() *C.char {
	defer logPanic()

	zap.L().Info("KeycardInitializeRPC - start")

	if err := checkAPIMutualExclusion(sessionAPI); err != nil {
		return marshalError(err)
	}

	rpcServer, err := session.CreateRPCServer()
	if err != nil {
		return marshalError(err)
	}
	globalRPCServer = rpcServer

	zap.L().Info("KeycardInitializeRPC - ok")
	return marshalError(nil)
}

//export KeycardCallRPC
func KeycardCallRPC(payload *C.char) *C.char {
	defer logPanic()

	zap.L().Info("Calling RPC", zap.String("payload", C.GoString(payload)))

	if globalRPCServer == nil {
		return marshalError(errors.New("RPC server not initialized"))
	}

	payloadBytes := []byte(C.GoString(payload))

	// Create a fake HTTP request
	req := httptest.NewRequest("POST", "/rpc", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	// Create a fake HTTP response writer
	rr := httptest.NewRecorder()

	zap.L().Info("ServeHTTP call")

	// Call the server's ServeHTTP method
	globalRPCServer.ServeHTTP(rr, req)

	zap.L().Info("ServeHTTP ok")

	// Read and return the response body
	resp := rr.Result()
	defer resp.Body.Close()

	zap.L().Info("ServeHTTP resp", zap.String("status", resp.Status))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return marshalError(errors.Wrap(err, "internal error reading response body"))
	}

	zap.L().Info("KeycardCallRPC returning", zap.String("body", string(body)))

	return C.CString(string(body))
}
