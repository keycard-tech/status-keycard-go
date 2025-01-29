package session

import (
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

var (
	globalKeycardService KeycardService
)

func CreateRPCServer() (*rpc.Server, error) {
	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(json.NewCodec(), "application/json")
	err := rpcServer.RegisterTCPService(&globalKeycardService, "keycard")
	return rpcServer, err
}
