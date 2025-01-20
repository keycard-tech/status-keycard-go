package mocked

import (
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
	"github.com/status-im/status-keycard-go/signal"
)

func (mkf *MockedKeycardFlow) handleChangePukFlow() {
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
		flow.InstanceUID: mkf.insertedKeycard.InstanceUID,
		flow.KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	if mkf.insertedKeycard.InstanceUID == "" && mkf.insertedKeycard.KeyUID == "" {
		flowStatus[internal.ErrorKey] = internal.ErrorRequireInit
		flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		mkf.state = flow.Paused
		signal.Send(flow.EnterNewPIN, flowStatus)
		return
	}

	if mkf.insertedKeycard.FreePairingSlots == 0 {
		flowStatus[internal.ErrorKey] = flow.FreeSlots
		flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		mkf.state = flow.Paused
		signal.Send(flow.SwapCard, flowStatus)
		return
	}

	var (
		enteredPIN    string
		enteredNewPIN string
		enteredPUK    string
		enteredNewPUK string
		overwrite     bool
	)

	if v, ok := mkf.params[flow.PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[flow.NewPIN]; ok {
		enteredNewPIN = v.(string)
	}
	if v, ok := mkf.params[flow.PUK]; ok {
		enteredPUK = v.(string)
	}
	if v, ok := mkf.params[flow.NewPUK]; ok {
		enteredNewPUK = v.(string)
	}
	if v, ok := mkf.params[flow.Overwrite]; ok {
		overwrite = v.(bool)
	}

	finalType := flow.EnterPIN
	if mkf.insertedKeycard.PukRetries == 0 {
		flowStatus[internal.ErrorKey] = flow.PUKRetries
		finalType = flow.SwapCard
	} else {
		if mkf.insertedKeycard.PinRetries == 0 {
			if len(enteredPUK) == flow.DefPUKLen {
				if len(enteredPIN) == flow.DefPINLen && enteredPIN == enteredNewPIN {
					if enteredPUK != mkf.insertedKeycard.Puk {
						mkf.insertedKeycard.PukRetries--
						if mkf.insertedKeycard.PukRetries == 0 {
							flowStatus[internal.ErrorKey] = flow.PUKRetries
							finalType = flow.SwapCard
						} else {
							flowStatus[internal.ErrorKey] = flow.PUK
							finalType = flow.EnterPUK
						}
					}
				} else {
					flowStatus[internal.ErrorKey] = internal.ErrorUnblocking
					finalType = flow.EnterNewPIN
				}
			} else {
				flowStatus[internal.ErrorKey] = ""
				finalType = flow.EnterPUK
			}
		} else {
			if len(enteredNewPIN) == 0 && len(enteredPIN) == flow.DefPINLen && enteredPIN != mkf.insertedKeycard.Pin {
				mkf.insertedKeycard.PinRetries--
				flowStatus[internal.ErrorKey] = flow.PIN
				finalType = flow.EnterPIN
				if mkf.insertedKeycard.PinRetries == 0 {
					flowStatus[internal.ErrorKey] = ""
					finalType = flow.EnterPUK
				}
			}
		}
	}

	if mkf.insertedKeycard.PinRetries > 0 && len(enteredPIN) == flow.DefPINLen && enteredPIN == mkf.insertedKeycard.Pin ||
		mkf.insertedKeycard.PinRetries == 0 && mkf.insertedKeycard.PukRetries > 0 && len(enteredPUK) == flow.DefPUKLen &&
			enteredPUK == mkf.insertedKeycard.Puk && len(enteredPIN) == flow.DefPINLen && enteredPIN == enteredNewPIN {
		if len(enteredNewPUK) == 0 {
			mkf.insertedKeycard.PinRetries = flow.MaxPINRetries
			mkf.insertedKeycard.PukRetries = flow.MaxPUKRetries
			mkf.insertedKeycard.Pin = enteredPIN
			flowStatus[internal.ErrorKey] = internal.ErrorChanging
			finalType = flow.EnterNewPUK
		} else if overwrite && len(enteredPUK) == flow.DefPUKLen && enteredPUK == enteredNewPUK {
			flowStatus[internal.ErrorKey] = ""
			mkf.insertedKeycard.PinRetries = flow.MaxPINRetries
			mkf.insertedKeycard.PukRetries = flow.MaxPUKRetries
			mkf.insertedKeycard.Puk = enteredPUK
			mkf.state = flow.Idle
			signal.Send(flow.FlowResult, flowStatus)
			return
		}
	}

	flowStatus[flow.FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[flow.PINRetries] = mkf.insertedKeycard.PinRetries
	flowStatus[flow.PUKRetries] = mkf.insertedKeycard.PukRetries
	mkf.state = flow.Paused
	signal.Send(finalType, flowStatus)
}
