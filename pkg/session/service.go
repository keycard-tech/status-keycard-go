package session

import (
	"github.com/status-im/status-keycard-go/internal"
	"github.com/pkg/errors"
	"github.com/status-im/status-keycard-go/pkg/utils"
)

var (
	errKeycardServiceNotStarted = errors.New("keycard service not started")
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

type InitializeKeycardRequest struct {
	PIN             string `json:"pin"`
	PUK             string `json:"puk"`
	PairingPassword string `json:"pairingPassword"`
}

func (s *KeycardService) InitializeKeycard(args *InitializeKeycardRequest, reply *struct{}) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	if args.PairingPassword == "" {
		args.PairingPassword = internal.DefPairing
	}

	err := s.keycardContext.InitializeKeycard(args.PIN, args.PUK, args.PairingPassword)
	return err
}

type VerifyPINRequest struct {
	PIN string `json:"pin"`
}

type VerifyPINResponse struct {
	PINCorrect bool `json:"pinCorrect"`
}

func (s *KeycardService) VerifyPIN(args *VerifyPINRequest, reply *VerifyPINResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	err := s.keycardContext.VerifyPIN(args.PIN)
	if err != nil {
		return err
	}
	reply.PINCorrect = true
	return nil
}

type GenerateSeedPhraseRequest struct {
	Length int `json:"length"`
}

type GenerateSeedPhraseResponse struct {
	Indexes []int `json:"indexes"`
}

func (s *KeycardService) GenerateSeedPhrase(args *GenerateSeedPhraseRequest, reply *GenerateSeedPhraseResponse) error {
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
	Mnemonic   string `json:"mnemonic"`
	Passphrase string `json:"passphrase"`
}

type LoadMnemonicResponse struct {
	KeyUID string `json:"keyUID"` // WARNING: Is this what's returned?
}

func (s *KeycardService) LoadMnemonic(args *LoadMnemonicRequest, reply *LoadMnemonicResponse) error {
	if s.keycardContext == nil {
		return errKeycardServiceNotStarted
	}

	keyUID, err := s.keycardContext.LoadMnemonic(args.Mnemonic, args.Passphrase)
	if err != nil {
		reply.KeyUID = utils.Btox(keyUID)
	}

	return err
}
