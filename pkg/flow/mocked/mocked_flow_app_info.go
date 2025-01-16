package mocked

import (
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
	"github.com/status-im/status-keycard-go/signal"
)

func (mkf *MockedKeycardFlow) handleGetAppInfoFlow() {
	flowStatus := flow.FlowStatus{}

	if mkf.insertedKeycard.NotStatusKeycard {
		flowStatus[internal.ErrorKey] = internal.ErrorNotAKeycard
		flowStatus[flow.InstanceUID] = ""
		flowStatus[flow.KeyUID] = ""
		flowStatus[flow.FreeSlots] = 0
		mkf.state = flow.Paused
		signal.Send(flow.SwapCard, flowStatus)
		return
	}

	flowStatus = flow.FlowStatus{
		flow.PINRetries: mkf.insertedKeycard.PinRetries,
		flow.PUKRetries: mkf.insertedKeycard.PukRetries,
	}

	if mkf.insertedKeycard.InstanceUID == "" || mkf.insertedKeycard.KeyUID == "" {
		flowStatus[internal.ErrorKey] = internal.ErrorNoKeys
		flowStatus[flow.FreeSlots] = 0
		mkf.state = flow.Paused
		signal.Send(flow.SwapCard, flowStatus)
		return
	}

	var (
		enteredPIN   string
		factoryReset bool
	)

	if v, ok := mkf.params[flow.PIN]; ok {
		enteredPIN = v.(string)
	}

	if v, ok := mkf.params[flow.FactoryReset]; ok {
		factoryReset = v.(bool)
	}

	if factoryReset {
		mkf.state = flow.Idle
		*mkf.insertedKeycard = MockedKeycard{}
		signal.Send(flow.FlowResult, flow.FlowStatus{
			internal.ErrorKey: internal.ErrorOK,
			flow.Paired:       false,
			flow.AppInfo: internal.ApplicationInfo{
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
	if len(enteredPIN) == flow.DefPINLen && enteredPIN == mkf.insertedKeycard.Pin || !keycardStoresKeys {
		flowStatus[internal.ErrorKey] = internal.ErrorOK
		flowStatus[flow.Paired] = keycardStoresKeys
		flowStatus[flow.AppInfo] = internal.ApplicationInfo{
			Initialized:    keycardStoresKeys,
			InstanceUID:    internal.HexString(mkf.insertedKeycard.InstanceUID),
			Version:        123,
			AvailableSlots: mkf.insertedKeycard.FreePairingSlots,
			KeyUID:         internal.HexString(mkf.insertedKeycard.KeyUID),
		}
		mkf.state = flow.Idle
		signal.Send(flow.FlowResult, flowStatus)
		return
	}

	flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[flow.InstanceUID] = mkf.insertedKeycard.InstanceUID
	flowStatus[flow.KeyUID] = mkf.insertedKeycard.KeyUID
	mkf.state = flow.Paused
	signal.Send(flow.EnterPIN, flowStatus)
}
