package api

import (
	gorillajson "github.com/gorilla/rpc/json"
	"github.com/gorilla/rpc"
)

func CreateRPCServer() (*rpc.Server, error) {
	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(gorillajson.NewCodec(), "application/json")
	err := rpcServer.RegisterService(&GlobalKeycardService, "keycard")
	return rpcServer, err
}
