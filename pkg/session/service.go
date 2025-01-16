package session

import (
	"github.com/status-im/status-keycard-go/internal"
	"github.com/pkg/errors"
)

var globalKeycardService KeycardService

type KeycardService struct {
	keycardContext *internal.KeycardContextV2
}

type StartRequest struct {
	StorageFilePath string `json:"storageFilePath"`
}

func (s *KeycardService) Start(args *StartRequest, reply *struct{}) error {
	var err error
	globalKeycardService.keycardContext, err = internal.NewKeycardContextV2(args.StorageFilePath)
	return err
}

func (s *KeycardService) Stop(args *struct{}, reply *struct{}) error {
	globalKeycardService.keycardContext.Stop()
	return nil
}

type VerifyPINRequest struct {
	PIN string `json:"pin"`
}

type VerifyPINResponse struct {
	Success bool `json:"success"`
}

func (s *KeycardService) VerifyPIN(args *VerifyPINRequest, reply *VerifyPINResponse) error {
	if globalKeycardService.keycardContext == nil {
		return errors.New("keycard service not started")
	}

	err := globalKeycardService.keycardContext.VerifyPIN(args.PIN)
	if err != nil {
		return err
	}
	reply.Success = true
	return nil
}
