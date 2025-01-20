package mocked

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
	"github.com/status-im/status-keycard-go/pkg/pairing"
	"github.com/status-im/status-keycard-go/signal"
)

type MockedKeycardFlow struct {
	flowType flow.FlowType
	state    flow.RunState
	params   flow.FlowParams
	pairings *pairing.Store

	mockedKeycardsStoreFilePath string

	initialReaderState       MockedReaderState
	currentReaderState       MockedReaderState
	registeredKeycards       map[int]*MockedKeycard
	registeredKeycardHelpers map[int]*MockedKeycard

	insertedKeycard       *MockedKeycard
	insertedKeycardHelper *MockedKeycard // used to generate necessary responses in case a mocked keycard is not configured
}

func NewMockedFlow(storageDir string) (*MockedKeycardFlow, error) {
	p, err := pairing.NewStore(storageDir)
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

	err = flow.loadRegisteredKeycards()

	return flow, err
}

func (mkf *MockedKeycardFlow) Start(flowType flow.FlowType, params flow.FlowParams) error {
	if mkf.state != flow.Idle {
		return errors.New("already running")
	}

	mkf.flowType = flowType
	mkf.params = params
	mkf.state = flow.Running

	go mkf.runFlow()

	return nil
}

func (mkf *MockedKeycardFlow) Resume(params flow.FlowParams) error {
	if mkf.state != flow.Paused {
		return errors.New("only paused flows can be resumed")
	}

	if mkf.params == nil {
		mkf.params = flow.FlowParams{}
	}

	for k, v := range params {
		mkf.params[k] = v
	}

	go mkf.runFlow()

	return nil
}

func (mkf *MockedKeycardFlow) Cancel() error {

	if mkf.state == flow.Idle {
		return errors.New("cannot cancel idle flow")
	}

	mkf.state = flow.Idle
	mkf.params = nil

	return nil
}

func (mkf *MockedKeycardFlow) ReaderPluggedIn() error {
	mkf.currentReaderState = NoKeycard

	if mkf.state == flow.Running {
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

	if mkf.state == flow.Running {
		go mkf.runFlow()
	}

	return nil
}

func (mkf *MockedKeycardFlow) KeycardRemoved() error {
	mkf.currentReaderState = NoKeycard

	mkf.insertedKeycard = nil
	mkf.insertedKeycardHelper = nil

	if mkf.state == flow.Running {
		go mkf.runFlow()
	}

	return nil
}

func (mkf *MockedKeycardFlow) RegisterKeycard(cardIndex int, readerState MockedReaderState, keycardState MockedKeycardState,
	keycard *MockedKeycard, keycardHelper *MockedKeycard) error {
	mkf.state = flow.Idle
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
		signal.Send(flow.FlowResult, flow.FlowStatus{internal.ErrorKey: internal.ErrorNoReader})
		return
	case NoKeycard:
		signal.Send(flow.InsertCard, flow.FlowStatus{internal.ErrorKey: internal.ErrorConnection})
		return
	default:
		switch mkf.flowType {
		case flow.GetAppInfo:
			mkf.handleGetAppInfoFlow()
		case flow.RecoverAccount:
			mkf.handleRecoverAccountFlow()
		case flow.LoadAccount:
			mkf.handleLoadAccountFlow()
		case flow.Login:
			mkf.handleLoginFlow()
		case flow.ExportPublic:
			mkf.handleExportPublicFlow()
		case flow.ChangePIN:
			mkf.handleChangePinFlow()
		case flow.ChangePUK:
			mkf.handleChangePukFlow()
		case flow.StoreMetadata:
			mkf.handleStoreMetadataFlow()
		case flow.GetMetadata:
			mkf.handleGetMetadataFlow()
		}
	}

	if mkf.insertedKeycard.InstanceUID != "" {
		pairing := mkf.pairings.Get(mkf.insertedKeycard.InstanceUID)
		if pairing == nil {
			err := mkf.pairings.Store(mkf.insertedKeycard.InstanceUID, mkf.insertedKeycard.PairingInfo)
			if err != nil {
				internal.Printf("error storing pairing: %v", err)
			}
		}
	}

	err := mkf.storeRegisteredKeycards()
	if err != nil {
		internal.Printf("error storing registered keycards: %v", err)
	}
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

	err = os.WriteFile(mkf.mockedKeycardsStoreFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (mkf *MockedKeycardFlow) loadRegisteredKeycards() error {
	data, err := os.ReadFile(mkf.mockedKeycardsStoreFilePath)
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
