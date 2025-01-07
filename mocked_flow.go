package statuskeycardgo

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"

	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

type MockedKeycardFlow struct {
	flowType FlowType
	state    runState
	params   FlowParams
	pairings *pairingStore

	mockedKeycardsStoreFilePath string

	initialReaderState       MockedReaderState
	currentReaderState       MockedReaderState
	registeredKeycards       map[int]*MockedKeycard
	registeredKeycardHelpers map[int]*MockedKeycard

	insertedKeycard       *MockedKeycard
	insertedKeycardHelper *MockedKeycard // used to generate necessary responses in case a mocked keycard is not configured
}

func NewMockedFlow(storageDir string) (*MockedKeycardFlow, error) {
	p, err := newPairingStore(storageDir)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(storageDir)

	flow := &MockedKeycardFlow{
		initialReaderState:          NoReader,
		currentReaderState:          NoReader,
		registeredKeycards:          make(map[int]*MockedKeycard),
		registeredKeycardHelpers:    make(map[int]*MockedKeycard),
		pairings:                    p,
		mockedKeycardsStoreFilePath: filepath.Join(dir, "mocked_keycards.json"),
	}

	flow.loadRegisteredKeycards()

	return flow, nil
}

func (mkf *MockedKeycardFlow) Start(flowType FlowType, params FlowParams) error {
	if mkf.state != Idle {
		return errors.New("already running")
	}

	mkf.flowType = flowType
	mkf.params = params
	mkf.state = Running

	go mkf.runFlow()

	return nil
}

func (mkf *MockedKeycardFlow) Resume(params FlowParams) error {
	if mkf.state != Paused {
		return errors.New("only paused flows can be resumed")
	}

	if mkf.params == nil {
		mkf.params = FlowParams{}
	}

	for k, v := range params {
		mkf.params[k] = v
	}

	go mkf.runFlow()

	return nil
}

func (mkf *MockedKeycardFlow) Cancel() error {

	if mkf.state == Idle {
		return errors.New("cannot cancel idle flow")
	}

	mkf.state = Idle
	mkf.params = nil

	return nil
}

func (mkf *MockedKeycardFlow) ReaderPluggedIn() error {
	mkf.currentReaderState = NoKeycard

	if mkf.state == Running {
		go mkf.runFlow()
	}

	return nil
}

func (mkf *MockedKeycardFlow) ReaderUnplugged() error {
	mkf.currentReaderState = NoReader

	go mkf.runFlow()

	return nil
}

func (mkf *MockedKeycardFlow) KeycardInserted(cardIndex int) error {
	if mkf.registeredKeycards == nil || mkf.registeredKeycardHelpers == nil ||
		len(mkf.registeredKeycards) == 0 || len(mkf.registeredKeycardHelpers) == 0 ||
		mkf.registeredKeycards[cardIndex] == nil || mkf.registeredKeycardHelpers[cardIndex] == nil {
		return errors.New("no registered keycards")
	}

	mkf.currentReaderState = KeycardInserted

	mkf.insertedKeycard = mkf.registeredKeycards[cardIndex]
	mkf.insertedKeycardHelper = mkf.registeredKeycardHelpers[cardIndex]

	if mkf.state == Running {
		go mkf.runFlow()
	}

	return nil
}

func (mkf *MockedKeycardFlow) KeycardRemoved() error {
	mkf.currentReaderState = NoKeycard

	mkf.insertedKeycard = nil
	mkf.insertedKeycardHelper = nil

	if mkf.state == Running {
		go mkf.runFlow()
	}

	return nil
}

func (mkf *MockedKeycardFlow) RegisterKeycard(cardIndex int, readerState MockedReaderState, keycardState MockedKeycardState,
	keycard *MockedKeycard, keycardHelper *MockedKeycard) error {
	mkf.state = Idle
	mkf.params = nil

	newKeycard := &MockedKeycard{}
	*newKeycard = mockedKeycard
	newKeycardHelper := &MockedKeycard{}
	*newKeycardHelper = mockedKeycardHelper

	switch keycardState {
	case NotStatusKeycard:
		newKeycard.NotStatusKeycard = true
	case EmptyKeycard:
		newKeycard = &MockedKeycard{}
	case MaxPairingSlotsReached:
		newKeycard.FreePairingSlots = 0
	case MaxPINRetriesReached:
		newKeycard.PinRetries = 0
	case MaxPUKRetriesReached:
		newKeycard.PukRetries = 0
	case KeycardWithMnemonicOnly:
		newKeycard.Metadata = internal.Metadata{}
	case KeycardWithMnemonicAndMedatada:
		*newKeycard = mockedKeycard
	default:
		if keycard == nil || keycardHelper == nil {
			return errors.New("keycard and keycard helper must be provided if custom state is used, at least empty `{}`")
		}
		newKeycard = keycard
		newKeycardHelper = keycardHelper
	}

	mkf.registeredKeycards[cardIndex] = newKeycard
	mkf.registeredKeycardHelpers[cardIndex] = newKeycardHelper

	mkf.initialReaderState = readerState
	mkf.currentReaderState = readerState
	mkf.insertedKeycard = newKeycard
	mkf.insertedKeycardHelper = newKeycardHelper

	return mkf.storeRegisteredKeycards()
}

func (mkf *MockedKeycardFlow) runFlow() {
	switch mkf.currentReaderState {
	case NoReader:
		signal.Send(FlowResult, FlowStatus{internal.ErrorKey: internal.ErrorNoReader})
		return
	case NoKeycard:
		signal.Send(InsertCard, FlowStatus{internal.ErrorKey: internal.ErrorConnection})
		return
	default:
		switch mkf.flowType {
		case GetAppInfo:
			mkf.handleGetAppInfoFlow()
		case RecoverAccount:
			mkf.handleRecoverAccountFlow()
		case LoadAccount:
			mkf.handleLoadAccountFlow()
		case Login:
			mkf.handleLoginFlow()
		case ExportPublic:
			mkf.handleExportPublicFlow()
		case ChangePIN:
			mkf.handleChangePinFlow()
		case ChangePUK:
			mkf.handleChangePukFlow()
		case StoreMetadata:
			mkf.handleStoreMetadataFlow()
		case GetMetadata:
			mkf.handleGetMetadataFlow()
		}
	}

	if mkf.insertedKeycard.InstanceUID != "" {
		pairing := mkf.pairings.get(mkf.insertedKeycard.InstanceUID)
		if pairing == nil {
			mkf.pairings.store(mkf.insertedKeycard.InstanceUID, mkf.insertedKeycard.PairingInfo)
		}
	}

	mkf.storeRegisteredKeycards()
}

func (mkf *MockedKeycardFlow) storeRegisteredKeycards() error {
	data, err := json.Marshal(struct {
		RegisteredKeycards       map[int]*MockedKeycard
		RegisteredKeycardHelpers map[int]*MockedKeycard
	}{
		mkf.registeredKeycards,
		mkf.registeredKeycardHelpers,
	})
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(mkf.mockedKeycardsStoreFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (mkf *MockedKeycardFlow) loadRegisteredKeycards() error {
	data, err := ioutil.ReadFile(mkf.mockedKeycardsStoreFilePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &struct {
		RegisteredKeycards       map[int]*MockedKeycard
		RegisteredKeycardHelpers map[int]*MockedKeycard
	}{
		mkf.registeredKeycards,
		mkf.registeredKeycardHelpers,
	})
	if err != nil {
		return err
	}

	return nil
}
