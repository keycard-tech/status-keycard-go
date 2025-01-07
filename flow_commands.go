package statuskeycardgo

import (
	"errors"
	"io"
	"strings"

	"github.com/status-im/keycard-go/apdu"
	"github.com/status-im/keycard-go/derivationpath"
	ktypes "github.com/status-im/keycard-go/types"
	"github.com/status-im/status-keycard-go/internal"
)

func (f *KeycardFlow) factoryReset(kc *keycardContext) error {
	err := kc.factoryReset(true)

	if err == nil {
		delete(f.params, FactoryReset)
		return restartErr()
	} else if internal.IsSCardError(err) {
		return restartErr()
	} else {
		return err
	}
}

func (f *KeycardFlow) selectKeycard(kc *keycardContext) error {
	appInfo, err := kc.selectApplet()

	if err != nil {
		return restartErr()
	}

	f.cardInfo.instanceUID = internal.Btox(appInfo.InstanceUID)
	f.cardInfo.keyUID = internal.Btox(appInfo.KeyUID)
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

func (f *KeycardFlow) pair(kc *keycardContext) error {
	if f.cardInfo.freeSlots == 0 {
		return f.pauseAndRestart(SwapCard, FreeSlots)
	}

	pairingPass, ok := f.params[PairingPass]

	if !ok {
		pairingPass = DefPairing
	}

	pairing, err := kc.pair(pairingPass.(string))

	if err == nil {
		return f.pairings.store(f.cardInfo.instanceUID, internal.ToPairInfo(pairing))
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

func (f *KeycardFlow) initCard(kc *keycardContext) error {
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
		newPairing = DefPairing
	}

	err := kc.init(newPIN.(string), newPUK.(string), newPairing.(string))

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

func (f *KeycardFlow) openSC(kc *keycardContext, giveup bool) error {
	var pairing *internal.PairingInfo

	if !kc.cmdSet.ApplicationInfo.Initialized && !giveup {
		return f.initCard(kc)
	} else {
		pairing = f.pairings.get(f.cardInfo.instanceUID)
	}

	if pairing != nil {
		err := kc.openSecureChannel(pairing.Index, pairing.Key)

		if err == nil {
			appStatus, err := kc.getStatusApplication()

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

		f.pairings.delete(f.cardInfo.instanceUID)
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

func (f *KeycardFlow) unblockPIN(kc *keycardContext) error {
	if f.cardInfo.pukRetries == 0 {
		return f.pauseAndRestart(SwapCard, PUKRetries)
	}

	pukError := ""
	var err error

	newPIN, pinOK := f.params[NewPIN]
	puk, pukOK := f.params[PUK]

	if pinOK && pukOK {
		err = kc.unblockPIN(puk.(string), newPIN.(string))

		if err == nil {
			f.cardInfo.pinRetries = maxPINRetries
			f.cardInfo.pukRetries = maxPUKRetries
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

func (f *KeycardFlow) authenticate(kc *keycardContext) error {
	if f.cardInfo.pinRetries == 0 {
		// succesful unblock leaves the card authenticated
		return f.unblockPIN(kc)
	}

	pinError := ""

	if pin, ok := f.params[PIN]; ok {
		err := kc.verifyPin(pin.(string))

		if err == nil {
			f.cardInfo.pinRetries = maxPINRetries
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

func (f *KeycardFlow) openSCAndAuthenticate(kc *keycardContext, giveup bool) error {
	err := f.openSC(kc, giveup)

	if err != nil {
		return err
	}

	return f.authenticate(kc)
}

func (f *KeycardFlow) unpairCurrent(kc *keycardContext) error {
	err := kc.unpairCurrent()

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) unpair(kc *keycardContext, idx int) error {
	err := kc.unpair(uint8(idx))

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) removeKey(kc *keycardContext) error {
	err := kc.removeKey()

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) getMetadata(kc *keycardContext) (*internal.Metadata, error) {
	m, err := kc.getMetadata()

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

func (f *KeycardFlow) storeMetadata(kc *keycardContext) error {
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
		if !strings.HasPrefix(p.(string), walletRoothPath) {
			return errors.New("path must start with " + walletRoothPath)
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

	err = kc.storeMetadata(m)

	if internal.IsSCardError(err) {
		return restartErr()
	}

	return err
}

func (f *KeycardFlow) exportKey(kc *keycardContext, path string, onlyPublic bool) (*internal.KeyPair, error) {
	keyPair, err := kc.exportKey(true, path == masterPath, onlyPublic, path)

	if internal.IsSCardError(err) {
		return nil, restartErr()
	}

	return keyPair, err
}

func (f *KeycardFlow) exportBIP44Key(kc *keycardContext) (interface{}, error) {
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

func (f *KeycardFlow) loadKeys(kc *keycardContext) error {
	if mnemonic, ok := f.params[Mnemonic]; ok {
		keyUID, err := kc.loadMnemonic(mnemonic.(string), "")

		if internal.IsSCardError(err) {
			return restartErr()
		} else if err != nil {
			return err
		}

		f.cardInfo.keyUID = internal.Btox(keyUID)
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
			mnemonicLength = defMnemoLen
		}
	} else {
		mnemonicLength = defMnemoLen
	}

	indexes, err := kc.generateMnemonic(mnemonicLength / 3)

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

func (f *KeycardFlow) changePIN(kc *keycardContext) error {
	if newPIN, ok := f.params[NewPIN]; ok {
		err := kc.changePin(newPIN.(string))

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

func (f *KeycardFlow) changePUK(kc *keycardContext) error {
	if newPUK, ok := f.params[NewPUK]; ok {
		err := kc.changePuk(newPUK.(string))

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

func (f *KeycardFlow) changePairing(kc *keycardContext) error {
	if newPairing, ok := f.params[NewPairing]; ok {
		err := kc.changePairingPassword(newPairing.(string))

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

func (f *KeycardFlow) sign(kc *keycardContext) (*internal.Signature, error) {
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
		rawHash, err = internal.Xtob(hash.(string))
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

	signature, err := kc.signWithPath(rawHash, path.(string))

	if internal.IsSCardError(err) {
		return nil, restartErr()
	} else if err != nil {
		return nil, err
	}

	return internal.ToSignature(signature), nil
}
