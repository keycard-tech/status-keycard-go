package main

import "C"
import (
	"github.com/status-im/status-keycard-go/pkg/api"
	"encoding/json"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net/http/httptest"
	"io"
	"fmt"
	"bytes"
	"github.com/gorilla/rpc"
)

type response struct {
	Error string      `json:"error"`
	Data  interface{} `json:"data"`
}

func buildResponse(data interface{}, err error) string {
	resp := response{
		Data: data,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	output, err := json.Marshal(resp)
	if err != nil {
		zap.L().Error("failed to marshal response", zap.Error(err))
	}

	return string(output)
}

func unmarshal[T any](input string) (*T, error) {
	var output T
	err := json.Unmarshal([]byte(input), &output)
	return &output, errors.Wrap(err, "failed to unmarshal request")
}

//export Start
func Start(request string) string {
	args, err := unmarshal[api.StartRequest](request)
	if err != nil {
		return buildResponse(nil, err)
	}
	var reply struct{}
	err = api.GlobalKeycardService.Start(nil, args, &reply)
	return buildResponse(reply, err)
}

//export Stop
func Stop() string {
	var args, reply struct{}
	err := api.GlobalKeycardService.Stop(nil, &args, &reply)
	return buildResponse(reply, err)
}

//export VerifyPIN
func VerifyPIN(request string) string {
	args, err := unmarshal[api.VerifyPINRequest](request)
	if err != nil {
		return buildResponse(nil, err)
	}
	var reply api.VerifyPINResponse
	err = api.GlobalKeycardService.VerifyPIN(nil, args, &reply)
	return buildResponse(reply, err)
}

var globalRPCServer *rpc.Server

//export KeycardInitializeRPC
func KeycardInitializeRPC() *C.char {
	zap.L().Info("Initializing RPC server - 1")

	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("failed to initialize log: %v\n", err)
	}
	zap.ReplaceGlobals(logger)

	zap.L().Info("Initializing RPC server - 2")

	rpcServer, err := api.CreateRPCServer()
	if err != nil {
		return C.CString(err.Error())
	}
	globalRPCServer = rpcServer
	logger.Info("RPC server initialized")
	return C.CString("")
}

//export KeycardCallRPC
func KeycardCallRPC(payload *C.char) *C.char {
	logger := zap.L().Named("KeycardCallRPC")
	logger.Debug("KeycardCallRPC", zap.String("payload", C.GoString(payload)))

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
	logger.Debug("KeycardCallRPC response", zap.String("body", string(body)))
	if err != nil {
		return C.CString(fmt.Sprintf("Error reading response: %v", err))
	}

	return C.CString(string(body))
}
