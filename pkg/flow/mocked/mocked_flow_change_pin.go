package mocked

import (
	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
	"github.com/status-im/status-keycard-go/pkg/flow"
)

func (mkf *MockedKeycardFlow) handleChangePinFlow() {
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
	if v, ok := mkf.params[flow.Overwrite]; ok {
		overwrite = v.(bool)
	}

	finalType := flow.EnterPIN
	if mkf.insertedKeycard.PukRetries == 0 {
		flowStatus[internal.ErrorKey] = flow.PUKRetries
		finalType = flow.SwapCard
	} else {
		if mkf.insertedKeycard.PinRetries == 0 {
			if len(enteredPUK) == internal.DefPUKLen {
				if len(enteredPIN) == internal.DefPINLen && enteredPIN == enteredNewPIN {
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
			if len(enteredNewPIN) == 0 && len(enteredPIN) == internal.DefPINLen && enteredPIN != mkf.insertedKeycard.Pin {
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

	if len(enteredNewPIN) == 0 {
		if mkf.insertedKeycard.PinRetries > 0 && len(enteredPIN) == internal.DefPINLen && enteredPIN == mkf.insertedKeycard.Pin {
			mkf.insertedKeycard.PinRetries = internal.MaxPINRetries
			mkf.insertedKeycard.PukRetries = internal.MaxPUKRetries
			mkf.insertedKeycard.Pin = enteredPIN
			flowStatus[internal.ErrorKey] = internal.ErrorChanging
			finalType = flow.EnterNewPIN
		}
	} else {
		if overwrite && len(enteredPIN) == internal.DefPINLen && enteredPIN == enteredNewPIN ||
			mkf.insertedKeycard.PinRetries == 0 && mkf.insertedKeycard.PukRetries > 0 && len(enteredPUK) == internal.DefPUKLen &&
				enteredPUK == mkf.insertedKeycard.Puk && len(enteredPIN) == internal.DefPINLen && enteredPIN == enteredNewPIN {
			mkf.insertedKeycard.PinRetries = internal.MaxPINRetries
			mkf.insertedKeycard.PukRetries = internal.MaxPUKRetries
			mkf.insertedKeycard.Pin = enteredPIN
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
