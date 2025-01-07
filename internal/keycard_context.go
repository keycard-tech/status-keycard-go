package statuskeycardgo

import (
	"crypto/sha512"
	"errors"
	"runtime"
	"time"

	"github.com/ebfe/scard"
	"github.com/ethereum/go-ethereum/crypto"
	keycard "github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/apdu"
	"github.com/status-im/keycard-go/globalplatform"
	"github.com/status-im/keycard-go/identifiers"
	"github.com/status-im/keycard-go/io"
	"github.com/status-im/keycard-go/types"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/text/unicode/norm"
	"github.com/status-im/status-keycard-go/internal"
)

const bip39Salt = "mnemonic"

type commandType int

const (
	Close commandType = iota
	Transmit
	Ack
)

type KeycardContext struct {
	cardCtx   *scard.Context
	card      *scard.Card
	readers   []string
	c         types.Channel
	cmdSet    *keycard.CommandSet
	connected chan (bool)
	command   chan (commandType)
	apdu      []byte
	rpdu      []byte
	runErr    error
}

func (kc *KeycardContext) Transmit(apdu []byte) ([]byte, error) {
	kc.apdu = apdu
	kc.command <- Transmit
	<-kc.command
	kc.apdu = nil
	rpdu, err := kc.rpdu, kc.runErr
	kc.rpdu = nil
	kc.runErr = nil
	return rpdu, err
}

func StartKeycardContext() (*KeycardContext, error) {
	kctx := &KeycardContext{
		connected: make(chan (bool)),
		command:   make(chan (commandType)),
	}

	go kctx.run()

	<-kctx.connected

	if kctx.runErr != nil {
		return nil, kctx.runErr
	}

	return kctx, nil
}

func (kc *KeycardContext) run() {
	runtime.LockOSThread()

	var err error

	defer func() {
		if err != nil {
			internal.Printf(err.Error())
		}

		kc.runErr = err

		if kc.cardCtx != nil {
			_ = kc.cardCtx.Release()
		}

		close(kc.connected)
		runtime.UnlockOSThread()
	}()

	err = kc.start()

	if err != nil {
		return
	}

	kc.connected <- true

	err = kc.connect()

	if err != nil {
		return
	}

	kc.connected <- true

	for cmd := range kc.command {
		switch cmd {
		case Transmit:
			kc.rpdu, kc.runErr = kc.card.Transmit(kc.apdu)
			kc.command <- Ack
		case Close:
			return
		}
	}
}

func (kc *KeycardContext) start() error {
	cardCtx, err := scard.EstablishContext()
	if err != nil {
		return errors.New(internal.ErrorPCSC)
	}

	internal.Printf("listing readers")
	readers, err := cardCtx.ListReaders()
	if err != nil {
		return errors.New(internal.ErrorReaderList)
	}

	kc.readers = readers

	if len(readers) == 0 {
		return errors.New(internal.ErrorNoReader)
	}

	kc.cardCtx = cardCtx
	return nil
}

func (kc *KeycardContext) Stop() {
	close(kc.command)
}

func (kc *KeycardContext) connect() error {
	internal.Printf("waiting for card")
	index, err := kc.waitForCard(kc.cardCtx, kc.readers)
	if err != nil {
		return err
	}

	internal.Printf("card found at index %d", index)
	reader := kc.readers[index]

	internal.Printf("using reader %s", reader)

	card, err := kc.cardCtx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		// error connecting to card
		time.Sleep(500 * time.Millisecond)
		return err
	}

	status, err := card.Status()
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		return err
	}

	switch status.ActiveProtocol {
	case scard.ProtocolT0:
		internal.Printf("card protocol T0")
	case scard.ProtocolT1:
		internal.Printf("card protocol T1")
	default:
		internal.Printf("card protocol T unknown")
	}

	kc.card = card
	kc.c = io.NewNormalChannel(kc)
	kc.cmdSet = keycard.NewCommandSet(kc.c)

	return nil
}

func (kc *KeycardContext) waitForCard(ctx *scard.Context, readers []string) (int, error) {
	rs := make([]scard.ReaderState, len(readers))

	for i := range rs {
		rs[i].Reader = readers[i]
		rs[i].CurrentState = scard.StateUnaware
	}

	for {
		for i := range rs {
			if rs[i].EventState&scard.StatePresent != 0 {
				return i, nil
			}

			rs[i].CurrentState = rs[i].EventState
		}

		err := ctx.GetStatusChange(rs, -1)
		if err != nil {
			return -1, err
		}
	}
}

func (kc *KeycardContext) SelectApplet() (*types.ApplicationInfo, error) {
	err := kc.cmdSet.Select()
	if err != nil {
		if e, ok := err.(*apdu.ErrBadResponse); ok && e.Sw == globalplatform.SwFileNotFound {
			err = nil
			kc.cmdSet.ApplicationInfo = &types.ApplicationInfo{}
		} else {
			internal.Printf("select failed %+v", err)
			return nil, err
		}
	}

	return kc.cmdSet.ApplicationInfo, nil
}

func (kc *KeycardContext) Pair(pairingPassword string) (*types.PairingInfo, error) {
	err := kc.cmdSet.Pair(pairingPassword)
	if err != nil {
		internal.Printf("Pair failed %+v", err)
		return nil, err
	}

	return kc.cmdSet.PairingInfo, nil
}

func (kc *KeycardContext) OpenSecureChannel(index int, key []byte) error {
	kc.cmdSet.SetPairingInfo(key, index)
	err := kc.cmdSet.OpenSecureChannel()
	if err != nil {
		internal.Printf("OpenSecureChannel failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) VerifyPin(pin string) error {
	err := kc.cmdSet.VerifyPIN(pin)
	if err != nil {
		internal.Printf("VerifyPin failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) UnblockPIN(puk string, newPIN string) error {
	err := kc.cmdSet.UnblockPIN(puk, newPIN)
	if err != nil {
		internal.Printf("UnblockPIN failed %+v", err)
		return err
	}

	return nil
}

//lint:ignore U1000 will be used
func (kc *KeycardContext) GenerateKey() ([]byte, error) {
	appStatus, err := kc.cmdSet.GetStatusApplication()
	if err != nil {
		internal.Printf("getStatus failed %+v", err)
		return nil, err
	}

	if appStatus.KeyInitialized {
		internal.Printf("generateKey failed - already generated - %+v", err)
		return nil, errors.New("key already generated")
	}

	keyUID, err := kc.cmdSet.GenerateKey()
	if err != nil {
		internal.Printf("generateKey failed %+v", err)
		return nil, err
	}

	return keyUID, nil
}

func (kc *KeycardContext) GenerateMnemonic(checksumSize int) ([]int, error) {
	indexes, err := kc.cmdSet.GenerateMnemonic(checksumSize)
	if err != nil {
		internal.Printf("generateMnemonic failed %+v", err)
		return nil, err
	}

	return indexes, nil
}

func (kc *KeycardContext) RemoveKey() error {
	err := kc.cmdSet.RemoveKey()
	if err != nil {
		internal.Printf("removeKey failed %+v", err)
		return err
	}

	return nil
}

//lint:ignore U1000 will be used
func (kc *KeycardContext) DeriveKey(path string) error {
	err := kc.cmdSet.DeriveKey(path)
	if err != nil {
		internal.Printf("deriveKey failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) SignWithPath(data []byte, path string) (*types.Signature, error) {
	sig, err := kc.cmdSet.SignWithPath(data, path)
	if err != nil {
		internal.Printf("signWithPath failed %+v", err)
		return nil, err
	}

	return sig, nil
}

func (kc *KeycardContext) ExportKey(derive bool, makeCurrent bool, onlyPublic bool, path string) (*internal.KeyPair, error) {
	address := ""
	privKey, pubKey, err := kc.cmdSet.ExportKey(derive, makeCurrent, onlyPublic, path)
	if err != nil {
		internal.Printf("exportKey failed %+v", err)
		return nil, err
	}

	if pubKey != nil {
		ecdsaPubKey, err := crypto.UnmarshalPubkey(pubKey)
		if err != nil {
			return nil, err
		}

		address = crypto.PubkeyToAddress(*ecdsaPubKey).Hex()
	}

	return &internal.KeyPair{Address: address, PublicKey: pubKey, PrivateKey: privKey}, nil
}

func (kc *KeycardContext) loadSeed(seed []byte) ([]byte, error) {
	pubKey, err := kc.cmdSet.LoadSeed(seed)
	if err != nil {
		internal.Printf("loadSeed failed %+v", err)
		return nil, err
	}

	return pubKey, nil
}

func (kc *KeycardContext) LoadMnemonic(mnemonic string, password string) ([]byte, error) {
	seed := pbkdf2.Key(norm.NFKD.Bytes([]byte(mnemonic)), norm.NFKD.Bytes([]byte(bip39Salt+password)), 2048, 64, sha512.New)
	return kc.loadSeed(seed)
}

func (kc *KeycardContext) Init(pin, puk, pairingPassword string) error {
	secrets := keycard.NewSecrets(pin, puk, pairingPassword)
	err := kc.cmdSet.Init(secrets)
	if err != nil {
		internal.Printf("init failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) unpair(index uint8) error {
	err := kc.cmdSet.Unpair(index)
	if err != nil {
		internal.Printf("unpair failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) UnpairCurrent() error {
	return kc.unpair(uint8(kc.cmdSet.PairingInfo.Index))
}

func (kc *KeycardContext) GetStatusApplication() (*types.ApplicationStatus, error) {
	status, err := kc.cmdSet.GetStatusApplication()
	if err != nil {
		internal.Printf("getStatusApplication failed %+v", err)
		return nil, err
	}

	return status, nil
}

func (kc *KeycardContext) ChangePin(pin string) error {
	err := kc.cmdSet.ChangePIN(pin)
	if err != nil {
		internal.Printf("chaingePin failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) ChangePuk(puk string) error {
	err := kc.cmdSet.ChangePUK(puk)
	if err != nil {
		internal.Printf("chaingePuk failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) ChangePairingPassword(pairingPassword string) error {
	err := kc.cmdSet.ChangePairingSecret(pairingPassword)
	if err != nil {
		internal.Printf("chaingePairingPassword failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) factoryResetFallback(retry bool) error {
	cmdSet := globalplatform.NewCommandSet(kc.c)

	if err := cmdSet.Select(); err != nil {
		internal.Printf("select ISD failed", "error", err)
		return err
	}

	if err := cmdSet.OpenSecureChannel(); err != nil {
		internal.Printf("open secure channel failed", "error", err)
		return err
	}

	aid, err := identifiers.KeycardInstanceAID(1)
	if err != nil {
		internal.Printf("error getting keycard aid %+v", err)
		return err
	}

	if err := cmdSet.DeleteObject(aid); err != nil {
		internal.Printf("error deleting keycard aid %+v", err)

		if retry {
			return kc.FactoryReset(false)
		} else {
			return err
		}
	}

	if err := cmdSet.InstallKeycardApplet(); err != nil {
		internal.Printf("error installing Keycard applet %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) FactoryReset(retry bool) error {
	appInfo, err := kc.SelectApplet()

	if err != nil || !appInfo.HasFactoryResetCapability() {
		return kc.factoryResetFallback(retry)
	}

	err = kc.cmdSet.FactoryReset()

	if err != nil {
		return kc.factoryResetFallback(retry)
	}

	return nil
}

func (kc *KeycardContext) StoreMetadata(metadata *types.Metadata) error {
	err := kc.cmdSet.StoreData(keycard.P1StoreDataPublic, metadata.Serialize())

	if err != nil {
		internal.Printf("storeMetadata failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) GetMetadata() (*types.Metadata, error) {
	data, err := kc.cmdSet.GetData(keycard.P1StoreDataPublic)

	if err != nil {
		internal.Printf("getMetadata failed %+v", err)
		return nil, err
	}

	return types.ParseMetadata(data)
}
