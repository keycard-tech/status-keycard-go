package statuskeycardgo

import (
	"errors"
	"time"

	"github.com/status-im/status-keycard-go/signal"
)

type cardStatus struct {
	instanceUID string
	keyUID      string
	freeSlots   int
	pinRetries  int
	pukRetries  int
}

type KeycardFlow struct {
	flowType FlowType
	state    runState
	wakeUp   chan (struct{})
	pairings *pairingStore
	params   FlowParams
	cardInfo cardStatus
}

func NewFlow(storageDir string) (*KeycardFlow, error) {
	p, err := newPairingStore(storageDir)

	if err != nil {
		l("error at os.ReadFile with path logged above is %+v", err)
		return nil, err
	}

	flow := &KeycardFlow{
		wakeUp:   make(chan (struct{})),
		pairings: p,
	}

	return flow, nil
}

func (f *KeycardFlow) Start(flowType FlowType, params FlowParams) error {
	if f.state != Idle {
		return errors.New("already running")
	}

	f.flowType = flowType
	f.params = params
	f.state = Running
	go f.runFlow()

	return nil
}

func (f *KeycardFlow) Resume(params FlowParams) error {
	if f.state != Paused {
		return errors.New("only paused flows can be resumed")
	}

	for k, v := range params {
		f.params[k] = v
	}

	f.state = Resuming
	f.wakeUp <- struct{}{}

	return nil
}

func (f *KeycardFlow) Cancel() error {
	prevState := f.state

	if prevState == Idle {
		return errors.New("cannot cancel idle flow")
	}

	f.state = Cancelling
	if prevState == Paused {
		f.wakeUp <- struct{}{}
	}

	return nil
}

func (f *KeycardFlow) runFlow() {
	var result FlowStatus
	var err error

	for {
		f.cardInfo = cardStatus{freeSlots: -1, pinRetries: -1, pukRetries: -1}
		result, err = f.connectedFlow()

		if _, ok := err.(*restartError); !ok {
			if result == nil {
				result = FlowStatus{ErrorKey: err.Error()}
				if f.cardInfo.freeSlots != -1 {
					result[InstanceUID] = f.cardInfo.instanceUID
					result[KeyUID] = f.cardInfo.keyUID
				}
			}
			break
		}
	}

	if f.state != Cancelling {
		signal.Send(FlowResult, result)
	}

	f.params = nil
	f.state = Idle
}

func (f *KeycardFlow) pause(action string, errMsg string, status FlowParams) {
	status[ErrorKey] = errMsg

	if f.cardInfo.freeSlots != -1 {
		status[InstanceUID] = f.cardInfo.instanceUID
		status[KeyUID] = f.cardInfo.keyUID
		status[FreeSlots] = f.cardInfo.freeSlots
	}

	if f.cardInfo.pinRetries != -1 {
		status[PINRetries] = f.cardInfo.pinRetries
		status[PUKRetries] = f.cardInfo.pukRetries
	}

	signal.Send(action, status)
	f.state = Paused
}

func (f *KeycardFlow) pauseAndWaitWithStatus(action string, errMsg string, status FlowParams) error {
	if f.state == Cancelling {
		return giveupErr()
	}

	f.pause(action, errMsg, status)
	<-f.wakeUp

	if f.state == Resuming {
		f.state = Running
		return nil
	} else {
		return giveupErr()
	}
}

func (f *KeycardFlow) pauseAndWait(action string, errMsg string) error {
	return f.pauseAndWaitWithStatus(action, errMsg, FlowParams{})
}

func (f *KeycardFlow) pauseAndRestart(action string, errMsg string) error {
	err := f.pauseAndWait(action, errMsg)

	if err != nil {
		return err
	}

	return restartErr()
}

func (f *KeycardFlow) requireKeys() error {
	if f.cardInfo.keyUID != "" {
		return nil
	}

	return f.pauseAndRestart(SwapCard, ErrorNoKeys)
}

func (f *KeycardFlow) requireNoKeys() error {
	if f.cardInfo.keyUID == "" {
		return nil
	}

	if overwrite, ok := f.params[Overwrite]; ok && overwrite.(bool) {
		return nil
	}

	return f.pauseAndRestart(SwapCard, ErrorHasKeys)
}

func (f *KeycardFlow) closeKeycard(kc *keycardContext) {
	if kc != nil {
		kc.stop()
	}
}

func (f *KeycardFlow) connect() (*keycardContext, error) {
	kc, err := startKeycardContext()

	if err != nil {
		return nil, err
	}

	t := time.NewTimer(150 * time.Millisecond)

	for {
		select {
		case <-f.wakeUp:
			if f.state != Cancelling {
				panic("Resuming is not expected during connection")
			}
			return nil, giveupErr()
		case <-kc.connected:
			if kc.runErr != nil {
				return nil, restartErr()
			}
			t.Stop()
			if f.state == Paused {
				f.state = Running
				signal.Send(CardInserted, FlowStatus{})
			}

			return kc, nil
		case <-t.C:
			f.pause(InsertCard, ErrorConnection, FlowParams{})
		}
	}
}

func (f *KeycardFlow) connectedFlow() (FlowStatus, error) {
	kc, err := f.connect()
	defer f.closeKeycard(kc)

	if err != nil {
		return nil, err
	}

	if factoryReset, ok := f.params[FactoryReset]; ok && factoryReset.(bool) {
		err := f.factoryReset(kc)

		if err != nil {
			return nil, err
		}
	}

	err = f.selectKeycard(kc)

	if err != nil {
		return nil, err
	}

	switch f.flowType {
	case GetAppInfo:
		return f.getAppInfoFlow(kc)
	case RecoverAccount:
		return f.exportKeysFlow(kc, true)
	case Login:
		return f.exportKeysFlow(kc, false)
	case ExportPublic:
		return f.exportPublicFlow(kc)
	case LoadAccount:
		return f.loadKeysFlow(kc)
	case Sign:
		return f.signFlow(kc)
	case ChangePIN:
		return f.changePINFlow(kc)
	case ChangePUK:
		return f.changePUKFlow(kc)
	case ChangePairing:
		return f.changePairingFlow(kc)
	case UnpairThis:
		return f.unpairThisFlow(kc)
	case UnpairOthers:
		return f.unpairOthersFlow(kc)
	case DeleteAccountAndUnpair:
		return f.deleteUnpairFlow(kc)
	case StoreMetadata:
		return f.storeMetadataFlow(kc)
	case GetMetadata:
		return f.getMetadataFlow(kc)
	default:
		return nil, errors.New(ErrorUnknownFlow)
	}
}

func (f *KeycardFlow) getAppInfoFlow(kc *keycardContext) (FlowStatus, error) {
	res := FlowStatus{ErrorKey: ErrorOK, AppInfo: toAppInfo(kc.cmdSet.ApplicationInfo)}
	err := f.openSCAndAuthenticate(kc, true)

	if err == nil {
		res[Paired] = true
		res[PINRetries] = f.cardInfo.pinRetries
		res[PUKRetries] = f.cardInfo.pukRetries
	} else if _, ok := err.(*giveupError); ok {
		res[Paired] = false
	} else {
		return nil, err
	}

	return res, nil
}

func (f *KeycardFlow) exportKeysFlow(kc *keycardContext, recover bool) (FlowStatus, error) {
	err := f.requireKeys()

	if err != nil {
		return nil, err
	}

	err = f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	result := FlowStatus{KeyUID: f.cardInfo.keyUID, InstanceUID: f.cardInfo.instanceUID}

	key, err := f.exportKey(kc, encryptionPath, false)
	if err != nil {
		return nil, err
	}
	result[EncKey] = key

	key, err = f.exportKey(kc, whisperPath, false)
	if err != nil {
		return nil, err
	}
	result[WhisperKey] = key

	if recover {
		key, err = f.exportKey(kc, eip1581Path, true)
		if err != nil {
			return nil, err
		}
		result[EIP1581Key] = key

		key, err = f.exportKey(kc, walletRoothPath, true)
		if err != nil {
			return nil, err
		}
		result[WalleRootKey] = key

		key, err = f.exportKey(kc, walletPath, true)
		if err != nil {
			return nil, err
		}
		result[WalletKey] = key

		key, err = f.exportKey(kc, masterPath, true)
		if err != nil {
			return nil, err
		}
		result[MasterKey] = key
	}

	return result, nil
}

func (f *KeycardFlow) exportPublicFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.requireKeys()

	if err != nil {
		return nil, err
	}

	err = f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	result := FlowStatus{KeyUID: f.cardInfo.keyUID, InstanceUID: f.cardInfo.instanceUID}

	if exportMaster, ok := f.params[ExportMaster]; ok && exportMaster.(bool) {
		masterKey, err := f.exportKey(kc, masterPath, true)
		result[MasterAddr] = masterKey.Address

		if err != nil {
			return nil, err
		}
	}

	key, err := f.exportBIP44Key(kc)

	if err != nil {
		return nil, err
	}

	result[ExportedKey] = key

	return result, nil
}

func (f *KeycardFlow) loadKeysFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.requireNoKeys()

	if err != nil {
		return nil, err
	}

	err = f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	err = f.loadKeys(kc)

	if err != nil {
		return nil, err
	}

	return FlowStatus{KeyUID: f.cardInfo.keyUID, InstanceUID: f.cardInfo.instanceUID}, nil
}

func (f *KeycardFlow) signFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.requireKeys()

	if err != nil {
		return nil, err
	}

	err = f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	signature, err := f.sign(kc)

	if err != nil {
		return nil, err
	}

	return FlowStatus{KeyUID: f.cardInfo.keyUID, InstanceUID: f.cardInfo.instanceUID, TXSignature: signature}, nil
}

func (f *KeycardFlow) changePINFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	err = f.changePIN(kc)

	if err != nil {
		return nil, err
	}

	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID}, nil
}

func (f *KeycardFlow) changePUKFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	err = f.changePUK(kc)

	if err != nil {
		return nil, err
	}

	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID}, nil
}

func (f *KeycardFlow) changePairingFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	err = f.changePairing(kc)

	if err != nil {
		return nil, err
	}

	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID}, nil
}

func (f *KeycardFlow) unpairThisFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, true)

	if err != nil {
		return nil, err
	}

	err = f.unpairCurrent(kc)

	if err != nil {
		return nil, err
	}

	f.cardInfo.freeSlots++
	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID, FreeSlots: f.cardInfo.freeSlots}, nil
}

func (f *KeycardFlow) unpairOthersFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, true)

	if err != nil {
		return nil, err
	}

	for i := 0; i < maxFreeSlots; i++ {
		if i == kc.cmdSet.PairingInfo.Index {
			continue
		}

		err = f.unpair(kc, i)

		if err != nil {
			return nil, err
		}

	}

	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID, FreeSlots: f.cardInfo.freeSlots}, nil
}

func (f *KeycardFlow) deleteUnpairFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, true)

	if err != nil {
		return nil, err
	}

	err = f.removeKey(kc)

	if err != nil {
		return nil, err
	}

	f.cardInfo.keyUID = ""

	err = f.unpairCurrent(kc)

	if err != nil {
		return nil, err
	}

	f.cardInfo.freeSlots++

	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID, FreeSlots: f.cardInfo.freeSlots}, nil
}

func (f *KeycardFlow) storeMetadataFlow(kc *keycardContext) (FlowStatus, error) {
	err := f.openSCAndAuthenticate(kc, false)

	if err != nil {
		return nil, err
	}

	err = f.storeMetadata(kc)

	if err != nil {
		return nil, err
	}

	return FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID}, nil
}

func (f *KeycardFlow) getMetadataFlow(kc *keycardContext) (FlowStatus, error) {
	m, err := f.getMetadata(kc)

	if err != nil {
		return nil, err
	}

	result := FlowStatus{InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID}

	if resolveAddr, ok := f.params[ResolveAddr]; ok && resolveAddr.(bool) {
		if f.cardInfo.keyUID == "" {
			return FlowStatus{ErrorKey: ErrorNoKeys, InstanceUID: f.cardInfo.instanceUID, KeyUID: f.cardInfo.keyUID, CardMeta: m}, nil
		}

		err := f.openSCAndAuthenticate(kc, false)

		if err != nil {
			return nil, err
		}

		if exportMaster, ok := f.params[ExportMaster]; ok && exportMaster.(bool) {
			masterKey, err := f.exportKey(kc, masterPath, true)
			result[MasterAddr] = masterKey.Address

			if err != nil {
				return nil, err
			}
		}

		for i := range m.Wallets {
			k, err := f.exportKey(kc, m.Wallets[i].Path, true)

			if err != nil {
				return nil, err
			}

			m.Wallets[i].Address = k.Address
			m.Wallets[i].PublicKey = k.PublicKey
		}
	}

	result[CardMeta] = m

	return result, nil
}
