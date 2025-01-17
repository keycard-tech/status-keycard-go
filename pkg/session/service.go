package session

import (
	goerrors "errors"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"

	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/utils"
)

var (
	errKeycardServiceNotStarted = errors.New("keycard service not started")
	validate                    = validator.New()
)

type KeycardService struct {
	keycardContext *internal.KeycardContextV2
}

type StartRequest struct {
	StorageFilePath string `json:"storageFilePath"`
}

func (s *KeycardService) Start(args *StartRequest, reply *struct{}) error {
	var err error
	s.keycardContext, err = internal.NewKeycardContextV2(args.StorageFilePath)
	return err
}

func (s *KeycardService) Stop(args *struct{}, reply *struct{}) error {
	s.keycardContext.Stop()
	s.keycardContext = nil
	return nil
}

// GetStatus should not be really used, as Status is pushed with `status-changed` signal.
// But it's handy to have for debugging purposes.
func (s *KeycardService) GetStatus(args *struct{}, reply *internal.Status) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	*reply = s.keycardContext.GetStatus()
	return nil
}

type InitializeRequest struct {
	PIN             string `json:"pin" validate:"required,len=6"`
	PUK             string `json:"puk" validate:"required,len=12"`
	PairingPassword string `json:"pairingPassword"`
}

func (s *KeycardService) Initialize(args *InitializeRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validate.Struct(args)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		return goerrors.Join(errs)
	}

	if args.PairingPassword == "" {
		args.PairingPassword = internal.DefPairing
	}

	err = s.keycardContext.Initialize(args.PIN, args.PUK, args.PairingPassword)
	return err
}

type ChangePINRequest struct {
	CurrentPIN string `json:"currentPin" validate:"required,len=6"`
	NewPIN     string `json:"newPin" validate:"required,len=6"`
}

func (s *KeycardService) ChangePIN(args *ChangePINRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validate.Struct(args)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		return goerrors.Join(errs)
	}

	err = s.keycardContext.VerifyPIN(args.CurrentPIN)
	if err != nil {
		return err
	}

	err = s.keycardContext.ChangePIN(args.NewPIN)
	return err
}

type VerifyPINRequest struct {
	PIN string `json:"pin" validate:"required,len=6"`
}

type VerifyPINResponse struct {
	PINCorrect bool `json:"pinCorrect"`
}

func (s *KeycardService) VerifyPIN(args *VerifyPINRequest, reply *VerifyPINResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := s.keycardContext.VerifyPIN(args.PIN)
	reply.PINCorrect = err == nil
	return err
}

type UnblockRequest struct {
	PUK    string `json:"puk" validate:"required,len=12"`
	NewPIN string `json:"newPin" validate:"required,len=6"`
}

func (s *KeycardService) Unblock(args *UnblockRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validate.Struct(args)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		return goerrors.Join(errs)
	}

	err = s.keycardContext.UnblockPIN(args.PUK, args.NewPIN)
	return err
}

type GenerateMnemonicRequest struct {
	Length int `json:"length"`
}

type GenerateMnemonicResponse struct {
	Indexes []int `json:"indexes"`
}

func (s *KeycardService) GenerateMnemonic(args *GenerateMnemonicRequest, reply *GenerateMnemonicResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	indexes, err := s.keycardContext.GenerateMnemonic(args.Length)
	if err != nil {
		return err
	}
	reply.Indexes = indexes
	return nil
}

type LoadMnemonicRequest struct {
	Mnemonic   string `json:"mnemonic" validate:"required"`
	Passphrase string `json:"passphrase"`
}

type LoadMnemonicResponse struct {
	KeyUID string `json:"keyUID"` // WARNING: Is this what's returned?
}

func (s *KeycardService) LoadMnemonic(args *LoadMnemonicRequest, reply *LoadMnemonicResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validate.Struct(args)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		return goerrors.Join(errs)
	}

	keyUID, err := s.keycardContext.LoadMnemonic(args.Mnemonic, args.Passphrase)
	reply.KeyUID = utils.Btox(keyUID)
	return err
}

func (s *KeycardService) FactoryReset(args *struct{}, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := s.keycardContext.FactoryReset()
	return err
}
