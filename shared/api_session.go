package main

import "C"
import (
	"github.com/status-im/status-keycard-go/pkg/session"
	"go.uber.org/zap"
	"net/http/httptest"
	"io"
	"fmt"
	"bytes"
	"github.com/gorilla/rpc"
)

var (
	globalRPCServer *rpc.Server
)

//export KeycardInitializeRPC
func KeycardInitializeRPC() *C.char {
	if err := checkAPIMutualExclusion(sessionAPI); err != nil {
		return C.CString(err.Error())
	}

	// TEMP: Replace with logging to a file, take the path as an argument
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("failed to initialize log: %v\n", err)
	}
	zap.ReplaceGlobals(logger)

	rpcServer, err := session.CreateRPCServer()
	if err != nil {
		return C.CString(err.Error())
	}
	globalRPCServer = rpcServer
	logger.Info("RPC server initialized")
	return C.CString("")
}

//export KeycardCallRPC
func KeycardCallRPC(payload *C.char) *C.char {
	if globalRPCServer == nil {
		return C.CString("RPC server not initialized")
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
		return C.CString(fmt.Sprintf("Error reading response: %v", err))
	}

	return C.CString(string(body))
}
