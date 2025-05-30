package session

import (
	"github.com/pkg/errors"

	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/pairing"
	"github.com/status-im/status-keycard-go/pkg/utils"
)

var (
	errKeycardServiceNotStarted = errors.New("keycard service not started")
)

type KeycardService struct {
	keycardContext *internal.KeycardContextV2
	simulateError  error
}

type StartRequest struct {
	// StorageFilePath is the path to the file where the keycard pairings information is stored.
	StorageFilePath string `json:"storageFilePath" validate:"required"`

	// LogEnabled is a flag to enable logging
	LogEnabled bool `json:"logEnabled,omitempty"`

	// LogFilePath is the path to the log file. When empty, logs go to stdout.
	LogFilePath string `json:"logFilePath,omitempty"`
}

func (s *KeycardService) Start(args *StartRequest, reply *struct{}) error {
	if s.keycardContext != nil {
		return errors.New("keycard service already started")
	}

	pairingsStore, err := pairing.NewStore(args.StorageFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to create pairing store")
	}

	options := []internal.Option{
		internal.WithStorage(pairingsStore),
		internal.WithLogging(args.LogEnabled, args.LogFilePath),
	}

	s.keycardContext, err = internal.NewKeycardContextV2(options)
	if err != nil {
		return err
	}

	err = s.keycardContext.SimulateError(s.simulateError)
	if err != nil {
		return err
	}

	return s.keycardContext.Start()
}

func (s *KeycardService) Stop(args *struct{}, reply *struct{}) error {
	if s.keycardContext == nil {
		return nil
	}
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

	err := validateRequest(args)
	if err != nil {
		return err
	}

	if args.PairingPassword == "" {
		args.PairingPassword = internal.DefPairing
	}

	err = s.keycardContext.Initialize(args.PIN, args.PUK, args.PairingPassword)
	return err
}

type AuthorizeRequest struct {
	PIN string `json:"pin" validate:"required,len=6"`
}

type AuthorizeResponse struct {
	Authorized bool `json:"authorized"`
}

func (s *KeycardService) Authorize(args *AuthorizeRequest, reply *AuthorizeResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err, authorized := s.keycardContext.VerifyPIN(args.PIN)
	reply.Authorized = authorized
	return err
}

type ChangePINRequest struct {
	NewPIN string `json:"newPin" validate:"required,len=6"`
}

func (s *KeycardService) ChangePIN(args *ChangePINRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validateRequest(args)
	if err != nil {
		return err
	}

	err = s.keycardContext.ChangePIN(args.NewPIN)
	return err
}

type ChangePUKRequest struct {
	NewPUK string `json:"newPuk" validate:"required,len=12"`
}

func (s *KeycardService) ChangePUK(args *ChangePUKRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validateRequest(args)
	if err != nil {
		return err
	}

	err = s.keycardContext.ChangePUK(args.NewPUK)
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

	err := validateRequest(args)
	if err != nil {
		return err
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
	Mnemonic   string `json:"mnemonic" validate:"required,mnemonic"`
	Passphrase string `json:"passphrase"`
}

type LoadMnemonicResponse struct {
	KeyUID string `json:"keyUID"` // WARNING: Is this what's returned?
}

func (s *KeycardService) LoadMnemonic(args *LoadMnemonicRequest, reply *LoadMnemonicResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validateRequest(args)
	if err != nil {
		return err
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

type GetMetadataResponse struct {
	Metadata *internal.Metadata `json:"metadata"`
}

func (s *KeycardService) GetMetadata(args *struct{}, reply *GetMetadataResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}
	var err error
	reply.Metadata, err = s.keycardContext.GetMetadata()
	return err
}

type StoreMetadataRequest struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

func (s *KeycardService) StoreMetadata(args *StoreMetadataRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := validateRequest(args)
	if err != nil {
		return err
	}

	return s.keycardContext.StoreMetadata(args.Name, args.Paths)
}

type ExportLoginKeysResponse struct {
	Keys *internal.LoginKeys `json:"keys"`
}

func (s *KeycardService) ExportLoginKeys(args *struct{}, reply *ExportLoginKeysResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	var err error
	reply.Keys, err = s.keycardContext.ExportLoginKeys()
	return err
}

type ExportRecoveredKeysResponse struct {
	Keys *internal.RecoverKeys `json:"keys"`
}

func (s *KeycardService) ExportRecoverKeys(args *struct{}, reply *ExportRecoveredKeysResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	var err error
	reply.Keys, err = s.keycardContext.ExportRecoverKeys()
	return err
}

type SimulateErrorRequest struct {
	Error string `json:"error"`
}

func (s *KeycardService) SimulateError(args *SimulateErrorRequest, reply *struct{}) error {
	err := validateRequest(args)
	if err != nil {
		return err
	}

	errToSimulate := internal.GetSimulatedError(args.Error)
	if args.Error != "" && errToSimulate == nil {
		return errors.New("unknown error to simulate")
	}

	s.simulateError = errToSimulate

	if s.keycardContext == nil {
		return nil
	}

	return s.keycardContext.SimulateError(errToSimulate)
}
