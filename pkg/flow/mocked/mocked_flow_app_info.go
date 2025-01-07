package flow

import (
	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleGetAppInfoFlow() {
	flowStatus := FlowStatus{}

	if mkf.insertedKeycard.NotStatusKeycard {
		flowStatus[internal.ErrorKey] = internal.ErrorNotAKeycard
		flowStatus[InstanceUID] = ""
		flowStatus[KeyUID] = ""
		flowStatus[FreeSlots] = 0
		mkf.state = Paused
		signal.Send(SwapCard, flowStatus)
		return
	}

	flowStatus = FlowStatus{
		PINRetries: mkf.insertedKeycard.PinRetries,
		PUKRetries: mkf.insertedKeycard.PukRetries,
	}

	if mkf.insertedKeycard.InstanceUID == "" || mkf.insertedKeycard.KeyUID == "" {
		flowStatus[internal.ErrorKey] = internal.ErrorNoKeys
		flowStatus[FreeSlots] = 0
		mkf.state = Paused
		signal.Send(SwapCard, flowStatus)
		return
	}

	var (
		enteredPIN   string
		factoryReset bool
	)

	if v, ok := mkf.params[PIN]; ok {
		enteredPIN = v.(string)
	}

	if v, ok := mkf.params[FactoryReset]; ok {
		factoryReset = v.(bool)
	}

	if factoryReset {
		mkf.state = Idle
		*mkf.insertedKeycard = MockedKeycard{}
		signal.Send(FlowResult, FlowStatus{
			internal.ErrorKey: internal.ErrorOK,
			Paired:            false,
			AppInfo: internal.ApplicationInfo{
				Initialized:    false,
				InstanceUID:    []byte(""),
				Version:        0,
				AvailableSlots: 0,
				KeyUID:         []byte(""),
			},
		})
		return
	}

	keycardStoresKeys := mkf.insertedKeycard.InstanceUID != "" && mkf.insertedKeycard.KeyUID != ""
	if len(enteredPIN) == defPINLen && enteredPIN == mkf.insertedKeycard.Pin || !keycardStoresKeys {
		flowStatus[internal.ErrorKey] = internal.ErrorOK
		flowStatus[Paired] = keycardStoresKeys
		flowStatus[AppInfo] = internal.ApplicationInfo{
			Initialized:    keycardStoresKeys,
			InstanceUID:    internal.HexString(mkf.insertedKeycard.InstanceUID),
			Version:        123,
			AvailableSlots: mkf.insertedKeycard.FreePairingSlots,
			KeyUID:         internal.HexString(mkf.insertedKeycard.KeyUID),
		}
		mkf.state = Idle
		signal.Send(FlowResult, flowStatus)
		return
	}

	flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[InstanceUID] = mkf.insertedKeycard.InstanceUID
	flowStatus[KeyUID] = mkf.insertedKeycard.KeyUID
	mkf.state = Paused
	signal.Send(EnterPIN, flowStatus)
}
