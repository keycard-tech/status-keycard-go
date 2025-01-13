package internal

import (
	"crypto/sha512"
	"errors"

	"github.com/status-im/keycard-go/types"
	"github.com/status-im/keycard-go/apdu"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/pbkdf2"
	"github.com/status-im/keycard-go"
	"github.com/status-im/keycard-go/globalplatform"
	"github.com/status-im/keycard-go/identifiers"
	"golang.org/x/text/unicode/norm"
)

func (kc *KeycardContext) SelectApplet() (*types.ApplicationInfo, error) {
	err := kc.cmdSet.Select()
	if err != nil {
		if e, ok := err.(*apdu.ErrBadResponse); ok && e.Sw == globalplatform.SwFileNotFound {
			err = nil
			kc.cmdSet.ApplicationInfo = &types.ApplicationInfo{}
		} else {
			Printf("select failed %+v", err)
			return nil, err
		}
	}

	return kc.cmdSet.ApplicationInfo, nil
}

func (kc *KeycardContext) Pair(pairingPassword string) (*types.PairingInfo, error) {
	err := kc.cmdSet.Pair(pairingPassword)
	if err != nil {
		Printf("Pair failed %+v", err)
		return nil, err
	}

	return kc.cmdSet.PairingInfo, nil
}

func (kc *KeycardContext) OpenSecureChannel(index int, key []byte) error {
	kc.cmdSet.SetPairingInfo(key, index)
	err := kc.cmdSet.OpenSecureChannel()
	if err != nil {
		Printf("OpenSecureChannel failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) VerifyPin(pin string) error {
	err := kc.cmdSet.VerifyPIN(pin)
	if err != nil {
		Printf("VerifyPin failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) UnblockPIN(puk string, newPIN string) error {
	err := kc.cmdSet.UnblockPIN(puk, newPIN)
	if err != nil {
		Printf("UnblockPIN failed %+v", err)
		return err
	}

	return nil
}

//lint:ignore U1000 will be used
func (kc *KeycardContext) GenerateKey() ([]byte, error) {
	appStatus, err := kc.cmdSet.GetStatusApplication()
	if err != nil {
		Printf("getStatus failed %+v", err)
		return nil, err
	}

	if appStatus.KeyInitialized {
		Printf("generateKey failed - already generated - %+v", err)
		return nil, errors.New("key already generated")
	}

	keyUID, err := kc.cmdSet.GenerateKey()
	if err != nil {
		Printf("generateKey failed %+v", err)
		return nil, err
	}

	return keyUID, nil
}

func (kc *KeycardContext) GenerateMnemonic(checksumSize int) ([]int, error) {
	indexes, err := kc.cmdSet.GenerateMnemonic(checksumSize)
	if err != nil {
		Printf("generateMnemonic failed %+v", err)
		return nil, err
	}

	return indexes, nil
}

func (kc *KeycardContext) RemoveKey() error {
	err := kc.cmdSet.RemoveKey()
	if err != nil {
		Printf("removeKey failed %+v", err)
		return err
	}

	return nil
}

//lint:ignore U1000 will be used
func (kc *KeycardContext) DeriveKey(path string) error {
	err := kc.cmdSet.DeriveKey(path)
	if err != nil {
		Printf("deriveKey failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) SignWithPath(data []byte, path string) (*types.Signature, error) {
	sig, err := kc.cmdSet.SignWithPath(data, path)
	if err != nil {
		Printf("signWithPath failed %+v", err)
		return nil, err
	}

	return sig, nil
}

func (kc *KeycardContext) ExportKey(derive bool, makeCurrent bool, onlyPublic bool, path string) (*KeyPair, error) {
	address := ""
	privKey, pubKey, err := kc.cmdSet.ExportKey(derive, makeCurrent, onlyPublic, path)
	if err != nil {
		Printf("exportKey failed %+v", err)
		return nil, err
	}

	if pubKey != nil {
		ecdsaPubKey, err := crypto.UnmarshalPubkey(pubKey)
		if err != nil {
			return nil, err
		}

		address = crypto.PubkeyToAddress(*ecdsaPubKey).Hex()
	}

	return &KeyPair{Address: address, PublicKey: pubKey, PrivateKey: privKey}, nil
}

func (kc *KeycardContext) loadSeed(seed []byte) ([]byte, error) {
	pubKey, err := kc.cmdSet.LoadSeed(seed)
	if err != nil {
		Printf("loadSeed failed %+v", err)
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
		Printf("init failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) Unpair(index uint8) error {
	err := kc.cmdSet.Unpair(index)
	if err != nil {
		Printf("Unpair failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) UnpairCurrent() error {
	return kc.Unpair(uint8(kc.cmdSet.PairingInfo.Index))
}

func (kc *KeycardContext) GetStatusApplication() (*types.ApplicationStatus, error) {
	status, err := kc.cmdSet.GetStatusApplication()
	if err != nil {
		Printf("getStatusApplication failed %+v", err)
		return nil, err
	}

	return status, nil
}

func (kc *KeycardContext) ChangePin(pin string) error {
	err := kc.cmdSet.ChangePIN(pin)
	if err != nil {
		Printf("chaingePin failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) ChangePuk(puk string) error {
	err := kc.cmdSet.ChangePUK(puk)
	if err != nil {
		Printf("chaingePuk failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) ChangePairingPassword(pairingPassword string) error {
	err := kc.cmdSet.ChangePairingSecret(pairingPassword)
	if err != nil {
		Printf("chaingePairingPassword failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) factoryResetFallback(retry bool) error {
	cmdSet := globalplatform.NewCommandSet(kc.c)

	if err := cmdSet.Select(); err != nil {
		Printf("select ISD failed", "error", err)
		return err
	}

	if err := cmdSet.OpenSecureChannel(); err != nil {
		Printf("open secure channel failed", "error", err)
		return err
	}

	aid, err := identifiers.KeycardInstanceAID(1)
	if err != nil {
		Printf("error getting keycard aid %+v", err)
		return err
	}

	if err := cmdSet.DeleteObject(aid); err != nil {
		Printf("error deleting keycard aid %+v", err)

		if retry {
			return kc.FactoryReset(false)
		} else {
			return err
		}
	}

	if err := cmdSet.InstallKeycardApplet(); err != nil {
		Printf("error installing Keycard applet %+v", err)
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
		Printf("storeMetadata failed %+v", err)
		return err
	}

	return nil
}

func (kc *KeycardContext) GetMetadata() (*types.Metadata, error) {
	data, err := kc.cmdSet.GetData(keycard.P1StoreDataPublic)

	if err != nil {
		Printf("getMetadata failed %+v", err)
		return nil, err
	}

	return types.ParseMetadata(data)
}
