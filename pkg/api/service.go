package api

import (
	"github.com/status-im/status-keycard-go/internal"
	"net/http"
	"github.com/pkg/errors"
)

var GlobalKeycardService KeycardService

type KeycardService struct {
	keycardContext *internal.KeycardContextV2
}

type StartRequest struct {
	StorageFilePath string `json:"storageFilePath"`
}

func (s *KeycardService) Start(r *http.Request, args *StartRequest, reply *struct{}) error {
	var err error
	GlobalKeycardService.keycardContext, err = internal.NewKeycardContextV2(args.StorageFilePath)
	return err
}

func (s *KeycardService) Stop(r *http.Request, args *struct{}, reply *struct{}) error {
	GlobalKeycardService.keycardContext.Stop()
	return nil
}

type VerifyPINRequest struct {
	PIN string `json:"pin"`
}

type VerifyPINResponse struct {
	Success bool `json:"success"`
}

func (s *KeycardService) VerifyPIN(r *http.Request, args *VerifyPINRequest, reply *VerifyPINResponse) error {
	if GlobalKeycardService.keycardContext == nil {
		return errors.New("keycard service not started")
	}

	err := GlobalKeycardService.keycardContext.VerifyPIN(args.PIN)
	if err != nil {
		return err
	}
	reply.Success = true
	return nil
}
