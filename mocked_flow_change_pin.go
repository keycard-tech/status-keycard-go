package statuskeycardgo

import (
	"github.com/status-im/status-keycard-go/signal"
	"github.com/status-im/status-keycard-go/internal"
)

func (mkf *MockedKeycardFlow) handleChangePinFlow() {
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
		InstanceUID: mkf.insertedKeycard.InstanceUID,
		KeyUID:      mkf.insertedKeycard.KeyUID,
	}

	if mkf.insertedKeycard.InstanceUID == "" && mkf.insertedKeycard.KeyUID == "" {
		flowStatus[internal.ErrorKey] = internal.ErrorRequireInit
		flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		mkf.state = Paused
		signal.Send(EnterNewPIN, flowStatus)
		return
	}

	if mkf.insertedKeycard.FreePairingSlots == 0 {
		flowStatus[internal.ErrorKey] = FreeSlots
		flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
		mkf.state = Paused
		signal.Send(SwapCard, flowStatus)
		return
	}

	var (
		enteredPIN    string
		enteredNewPIN string
		enteredPUK    string
		overwrite     bool
	)

	if v, ok := mkf.params[PIN]; ok {
		enteredPIN = v.(string)
	}
	if v, ok := mkf.params[NewPIN]; ok {
		enteredNewPIN = v.(string)
	}
	if v, ok := mkf.params[PUK]; ok {
		enteredPUK = v.(string)
	}
	if v, ok := mkf.params[Overwrite]; ok {
		overwrite = v.(bool)
	}

	finalType := EnterPIN
	if mkf.insertedKeycard.PukRetries == 0 {
		flowStatus[internal.ErrorKey] = PUKRetries
		finalType = SwapCard
	} else {
		if mkf.insertedKeycard.PinRetries == 0 {
			if len(enteredPUK) == defPUKLen {
				if len(enteredPIN) == defPINLen && enteredPIN == enteredNewPIN {
					if enteredPUK != mkf.insertedKeycard.Puk {
						mkf.insertedKeycard.PukRetries--
						if mkf.insertedKeycard.PukRetries == 0 {
							flowStatus[internal.ErrorKey] = PUKRetries
							finalType = SwapCard
						} else {
							flowStatus[internal.ErrorKey] = PUK
							finalType = EnterPUK
						}
					}
				} else {
					flowStatus[internal.ErrorKey] = internal.ErrorUnblocking
					finalType = EnterNewPIN
				}
			} else {
				flowStatus[internal.ErrorKey] = ""
				finalType = EnterPUK
			}
		} else {
			if len(enteredNewPIN) == 0 && len(enteredPIN) == defPINLen && enteredPIN != mkf.insertedKeycard.Pin {
				mkf.insertedKeycard.PinRetries--
				flowStatus[internal.ErrorKey] = PIN
				finalType = EnterPIN
				if mkf.insertedKeycard.PinRetries == 0 {
					flowStatus[internal.ErrorKey] = ""
					finalType = EnterPUK
				}
			}
		}
	}

	if len(enteredNewPIN) == 0 {
		if mkf.insertedKeycard.PinRetries > 0 && len(enteredPIN) == defPINLen && enteredPIN == mkf.insertedKeycard.Pin {
			mkf.insertedKeycard.PinRetries = maxPINRetries
			mkf.insertedKeycard.PukRetries = maxPUKRetries
			mkf.insertedKeycard.Pin = enteredPIN
			flowStatus[internal.ErrorKey] = internal.ErrorChanging
			finalType = EnterNewPIN
		}
	} else {
		if overwrite && len(enteredPIN) == defPINLen && enteredPIN == enteredNewPIN ||
			mkf.insertedKeycard.PinRetries == 0 && mkf.insertedKeycard.PukRetries > 0 && len(enteredPUK) == defPUKLen &&
				enteredPUK == mkf.insertedKeycard.Puk && len(enteredPIN) == defPINLen && enteredPIN == enteredNewPIN {
			mkf.insertedKeycard.PinRetries = maxPINRetries
			mkf.insertedKeycard.PukRetries = maxPUKRetries
			mkf.insertedKeycard.Pin = enteredPIN
			signal.Send(FlowResult, flowStatus)
			return
		}
	}

	flowStatus[FreeSlots] = mkf.insertedKeycard.FreePairingSlots
	flowStatus[PINRetries] = mkf.insertedKeycard.PinRetries
	flowStatus[PUKRetries] = mkf.insertedKeycard.PukRetries
	mkf.state = Paused
	signal.Send(finalType, flowStatus)
}
