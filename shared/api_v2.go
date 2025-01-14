package main

import (
	"github.com/status-im/status-keycard-go/pkg/api"
	"encoding/json"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
