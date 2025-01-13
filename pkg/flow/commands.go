package flow

import (
	"errors"
	"io"
	"strings"

	"github.com/status-im/keycard-go/apdu"
	"github.com/status-im/keycard-go/derivationpath"
	ktypes "github.com/status-im/keycard-go/types"
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/utils"
	"github.com/status-im/status-keycard-go/pkg/pairing"
)

func (f *KeycardFlow) factoryReset(kc *internal.KeycardContext) error {
	err := kc.FactoryReset(true)

	if err == nil {
		delete(f.params, FactoryReset)
		return restartErr()
	} else if internal.IsSCardError(err) {
		return restartErr()
	} else {
		return err
	}
}

func (f *KeycardFlow) selectKeycard(kc *internal.KeycardContext) error {
	appInfo, err := kc.SelectApplet()

	if err != nil {
		return restartErr()
	}

	f.cardInfo.instanceUID = utils.Btox(appInfo.InstanceUID)
	f.cardInfo.keyUID = utils.Btox(appInfo.KeyUID)
	f.cardInfo.freeSlots = internal.BytesToInt(appInfo.AvailableSlots)

	if !appInfo.Installed {
		return f.pauseAndRestart(SwapCard, internal.ErrorNotAKeycard)
	}

	if requiredInstanceUID, ok := f.params[InstanceUID]; ok {
		if f.cardInfo.instanceUID != requiredInstanceUID {
			return f.pauseAndRestart(SwapCard, InstanceUID)
		}
	}

	if requiredKeyUID, ok := f.params[KeyUID]; ok {
		if f.cardInfo.keyUID != requiredKeyUID {
			return f.pauseAndRestart(SwapCard, KeyUID)
		}
	}

	return nil
}

func (f *KeycardFlow) pair(kc *internal.KeycardContext) error {
	if f.cardInfo.freeSlots == 0 {
		return f.pauseAndRestart(SwapCard, FreeSlots)
	}

	pairingPass, ok := f.params[PairingPass]

	if !ok {
		pairingPass = internal.DefPairing
	}

	pair, err := kc.Pair(pairingPass.(string))

	if err == nil {
		return f.pairings.Store(f.cardInfo.instanceUID, pairing.ToPairInfo(pair))
	} else if internal.IsSCardError(err) {
		return restartErr()
	}

	delete(f.params, PairingPass)

	err = f.pauseAndWait(EnterPairing, internal.ErrorPairing)

	if err != nil {
		return err
	}

	return f.pair(kc)
}

func (f *KeycardFlow) initCard(kc *internal.KeycardContext) error {
	newPIN, pinOK := f.params[NewPIN]

	if !pinOK {
		err := f.pauseAndWait(EnterNewPIN, internal.ErrorRequireInit)
		if err != nil {
			return err
		}

		return f.initCard(kc)
	}

	newPUK, pukOK := f.params[NewPUK]
	if !pukOK {
		err := f.pauseAndWait(EnterNewPUK, internal.ErrorRequireInit)
		if err != nil {
			return err
		}

		return f.initCard(kc)
	}

	newPairing, pairingOK := f.params[NewPairing]
	if !pairingOK {
		newPairing = internal.DefPairing
	}

	err := kc.Init(newPIN.(string), newPUK.(string), newPairing.(string))

	if internal.IsSCardError(err) {
		return restartErr()
	} else if err != nil {
		return err
	}

	f.params[PIN] = newPIN
	f.params[PairingPass] = newPairing
	delete(f.params, NewPIN)
	delete(f.params, NewPUK)
	delete(f.params, NewPairing)

	return restartErr()
}

func (f *KeycardFlow) openSC(kc *internal.KeycardContext, giveup bool) error {
	var pairing *pairing.Info

	if !kc.ApplicationInfo().Initialized && !giveup {
		return f.initCard(kc)
	} else {
		pairing = f.pairings.Get(f.cardInfo.instanceUID)
	}

	if pairing != nil {
		err := kc.OpenSecureChannel(pairing.Index, pairing.Key)

		if err == nil {
			appStatus, err := kc.GetStatusApplication()

			if err != nil {
				// getStatus can only fail for connection errors
				return restartErr()
			}

			f.cardInfo.pinRetries = appStatus.PinRetryCount
			f.cardInfo.pukRetries = appStatus.PUKRetryCount

			return nil
		} else if internal.IsSCardError(err) {
			return restartErr()
		}

		f.pairings.Delete(f.cardInfo.instanceUID)
	}

	if giveup {
		return giveupErr()
	}

	err := f.pair(kc)

	if err != nil {
		return err
	}

	return f.openSC(kc, giveup)
}

func (f *KeycardFlow) unblockPIN(kc *internal.KeycardContext) error {
	if f.cardInfo.pukRetries == 0 {
		return f.pauseAndRestart(SwapCard, PUKRetries)
	}

	pukError := ""
	var err error

	newPIN, pinOK := f.params[NewPIN]
	puk, pukOK := f.params[PUK]

	if pinOK && pukOK {
		err = kc.UnblockPIN(puk.(string), newPIN.(string))

		if err == nil {
			f.cardInfo.pinRetries = internal.MaxPINRetries
			f.cardInfo.pukRetries = internal.MaxPUKRetries
			f.params[PIN] = newPIN
			delete(f.params, NewPIN)
			delete(f.params, PUK)
			return nil
		} else if internal.IsSCardError(err) {
			return restartErr()
		} else if leftRetries, ok := internal.GetRetries(err); ok {
			f.cardInfo.pukRetries = leftRetries
			delete(f.params, PUK)
			pukOK = false
		}

		pukError = PUK
	}

	if f.cardInfo.pukRetries == 0 {
		return f.pauseAndRestart(SwapCard, PUKRetries)
	}

	if !pukOK {
		err = f.pauseAndWait(EnterPUK, pukError)
	} else if !pinOK {
		err = f.pauseAndWait(EnterNewPIN, internal.ErrorUnblocking)
	}

	if err != nil {
		return err
	}

	return f.unblockPIN(kc)
}

func (f *KeycardFlow) authenticate(kc *internal.KeycardContext) error {
	if f.cardInfo.pinRetries == 0 {
		// succesful unblock leaves the card authenticated
		return f.unblockPIN(kc)
	}

	pinError := ""

	if pin, ok := f.params[PIN]; ok {
		err := kc.VerifyPin(pin.(string))

		if err == nil {
			f.cardInfo.pinRetries = internal.MaxPINRetries
			return nil
		} else if internal.IsSCardError(err) {
			return restartErr()
		} else if leftRetries, ok := internal.GetRetries(err); ok {
			f.cardInfo.pinRetries = leftRetries
			delete(f.params, PIN)
		}

		pinError = PIN
	}

	if f.cardInfo.pinRetries == 0 {
		return f.unblockPIN(kc)
	}

	err := f.pauseAndWait(EnterPIN, pinError)

	if err != nil {
		return err
	}

	return f.authenticate(kc)
}

func (f *KeycardFlow) openSCAndAuthenticate(kc *internal.KeycardContext, giveup bool) error {
	err := f.openSC(kc, giveup)

	if err != nil {
		return err
	}

	return f.authenticate(kc)
}

func (f *KeycardFlow) unpairCurrent(kc *internal.KeycardContext) error {
	err := kc.UnpairCurrent()

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) unpair(kc *internal.KeycardContext, idx int) error {
	err := kc.Unpair(uint8(idx))

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) removeKey(kc *internal.KeycardContext) error {
	err := kc.RemoveKey()

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) getMetadata(kc *internal.KeycardContext) (*internal.Metadata, error) {
	m, err := kc.GetMetadata()

	if err == nil {
		return internal.ToMetadata(m), nil
	} else if internal.IsSCardError(err) {
		return nil, restartErr()
	} else if serr, ok := err.(*apdu.ErrBadResponse); ok {
		if serr.Sw == 0x6d00 {
			return nil, errors.New(internal.ErrorNoKeys)
		} else {
			return nil, err
		}
	} else if err == io.EOF {
		return nil, errors.New(internal.ErrorNoData)
	} else {
		return nil, err
	}
}

func (f *KeycardFlow) storeMetadata(kc *internal.KeycardContext) error {
	cardName, cardNameOK := f.params[CardName]

	if !cardNameOK {
		err := f.pauseAndWait(EnterName, internal.ErrorStoreMeta)
		if err != nil {
			return err
		}

		return f.storeMetadata(kc)
	}

	w, walletsOK := f.params[WalletPaths]

	if !walletsOK {
		err := f.pauseAndWait(EnterWallets, internal.ErrorStoreMeta)
		if err != nil {
			return err
		}

		return f.storeMetadata(kc)
	}

	wallets := w.([]interface{})

	paths := make([]uint32, len(wallets))
	for i, p := range wallets {
		if !strings.HasPrefix(p.(string), internal.WalletRoothPath) {
			return errors.New("path must start with " + internal.WalletRoothPath)
		}

		_, components, err := derivationpath.Decode(p.(string))
		if err != nil {
			return err
		}

		paths[i] = components[len(components)-1]
	}

	m, err := ktypes.NewMetadata(cardName.(string), paths)
	if err != nil {
		return err
	}

	err = kc.StoreMetadata(m)

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) exportKey(kc *internal.KeycardContext, path string, onlyPublic bool) (*internal.KeyPair, error) {
	keyPair, err := kc.ExportKey(true, path == internal.MasterPath, onlyPublic, path)

	if internal.IsSCardError(err) {
		return nil, restartErr()
	}

	return keyPair, err
}

func (f *KeycardFlow) exportBIP44Key(kc *internal.KeycardContext) (interface{}, error) {
	if path, ok := f.params[BIP44Path]; ok {
		exportPrivParam, ok := f.params[ExportPriv]
		exportPrivate := (!ok || !exportPrivParam.(bool))

		if pathStr, ok := path.(string); ok {
			return f.exportKey(kc, pathStr, exportPrivate)
		} else if paths, ok := path.([]interface{}); ok {
			keys := make([]*internal.KeyPair, len(paths))

			for i, path := range paths {
				key, err := f.exportKey(kc, path.(string), exportPrivate)
				if err != nil {
					return nil, err
				}

				keys[i] = key
			}

			return keys, nil
		} else {
			delete(f.params, BIP44Path)
			return f.exportBIP44Key(kc)
		}
	}

	err := f.pauseAndWait(EnterPath, internal.ErrorExporting)

	if err != nil {
		return nil, err
	}

	return f.exportBIP44Key(kc)
}

func (f *KeycardFlow) loadKeys(kc *internal.KeycardContext) error {
	if mnemonic, ok := f.params[Mnemonic]; ok {
		keyUID, err := kc.LoadMnemonic(mnemonic.(string), "")

		if internal.IsSCardError(err) {
			return restartErr()
		} else if err != nil {
			return err
		}

		f.cardInfo.keyUID = utils.Btox(keyUID)
		return nil
	}

	tmpMnemonic, ok := f.params[MnemonicLen]
	var mnemonicLength int
	if ok {
		switch t := tmpMnemonic.(type) {
		case int:
			mnemonicLength = t
		case float64:
			mnemonicLength = int(t)
		default:
			mnemonicLength = internal.DefMnemoLen
		}
	} else {
		mnemonicLength = internal.DefMnemoLen
	}

	indexes, err := kc.GenerateMnemonic(mnemonicLength / 3)

	if internal.IsSCardError(err) {
		return restartErr()
	} else if err != nil {
		return err
	}

	err = f.pauseAndWaitWithStatus(EnterMnemonic, internal.ErrorLoading, FlowParams{MnemonicIdxs: indexes})

	if err != nil {
		return err
	}

	return f.loadKeys(kc)
}

func (f *KeycardFlow) changePIN(kc *internal.KeycardContext) error {
	if newPIN, ok := f.params[NewPIN]; ok {
		err := kc.ChangePin(newPIN.(string))

		if internal.IsSCardError(err) {
			return restartErr()
		} else if err != nil {
			return err
		}

		return nil
	}

	err := f.pauseAndWait(EnterNewPIN, internal.ErrorChanging)

	if err != nil {
		return err
	}

	return f.changePIN(kc)
}

func (f *KeycardFlow) changePUK(kc *internal.KeycardContext) error {
	if newPUK, ok := f.params[NewPUK]; ok {
		err := kc.ChangePuk(newPUK.(string))

		if internal.IsSCardError(err) {
			return restartErr()
		} else if err != nil {
			return err
		}

		return nil
	}

	err := f.pauseAndWait(EnterNewPUK, internal.ErrorChanging)

	if err != nil {
		return err
	}

	return f.changePUK(kc)
}

func (f *KeycardFlow) changePairing(kc *internal.KeycardContext) error {
	if newPairing, ok := f.params[NewPairing]; ok {
		err := kc.ChangePairingPassword(newPairing.(string))

		if internal.IsSCardError(err) {
			return restartErr()
		} else if err != nil {
			return err
		}

		return nil
	}

	err := f.pauseAndWait(EnterNewPair, internal.ErrorChanging)

	if err != nil {
		return err
	}

	return f.changePairing(kc)
}

func (f *KeycardFlow) sign(kc *internal.KeycardContext) (*internal.Signature, error) {
	var err error

	path, pathOK := f.params[BIP44Path]

	if !pathOK {
		err = f.pauseAndWait(EnterPath, internal.ErrorSigning)
		if err != nil {
			return nil, err
		}

		return f.sign(kc)
	}

	hash, hashOK := f.params[TXHash]

	var rawHash []byte

	if hashOK {
		rawHash, err = utils.Xtob(hash.(string))
		if err != nil {
			hashOK = false
		}
	}

	if !hashOK {
		err := f.pauseAndWait(EnterTXHash, internal.ErrorSigning)
		if err != nil {
			return nil, err
		}

		return f.sign(kc)
	}

	signature, err := kc.SignWithPath(rawHash, path.(string))

	if internal.IsSCardError(err) {
		return nil, restartErr()
	} else if err != nil {
		return nil, err
	}

	return internal.ToSignature(signature), nil
}
