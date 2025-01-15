package main

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

//export InitializeRPC
func InitializeRPC() string {
	rpcServer, err := api.CreateRPCServer()
	if err != nil {
		return err.Error()
	}
	globalRPCServer = rpcServer
	return ""
}

//export CallRPC
func CallRPC(payload string) string {
	if globalRPCServer == nil {
		return "RPC server not initialized"
	}

	payloadBytes := []byte(payload)

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
		return fmt.Sprintf("Error reading response: %v", err)
	}

	return string(body)
}
